package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"gorm.io/gorm"
)

var (
	ErrRelationInvalid  = errors.New("invalid relation request")
	ErrRelationNotFound = errors.New("relation not found")
	// ErrRelationConflict 表示当前状态不允许该操作（如确认已忽略的建议）。
	ErrRelationConflict = errors.New("relation conflict")
)

// RelationConflictError 携带可直接展示给用户的中文冲突原因；
// errors.Is 仍能匹配 ErrRelationConflict。
type RelationConflictError struct{ Message string }

func (e *RelationConflictError) Error() string { return e.Message }

func (e *RelationConflictError) Is(target error) bool { return target == ErrRelationConflict }

func relationConflict(message string) error { return &RelationConflictError{Message: message} }

// RelationStore 是关系仓储的最小能力集合（*repository.RelationRepository 满足）。
type RelationStore interface {
	Create(context.Context, *model.KnowledgeRelation) (*model.KnowledgeRelation, error)
	Get(context.Context, uint, uint) (*repository.RelationRecord, error)
	ListByKnowledge(context.Context, uint, uint) ([]repository.RelationRecord, error)
	UpdateStatus(context.Context, uint, uint, model.RelationStatus, *time.Time) error
	UpdateType(context.Context, uint, uint, model.RelationType) (*model.KnowledgeRelation, error)
	Delete(context.Context, uint, uint) error
}

// RelationKnowledgeStore 用于校验关系两端的知识归属（*KnowledgeService 满足）。
type RelationKnowledgeStore interface {
	Get(context.Context, uint, uint) (*model.KnowledgeItem, error)
}

// RelationTaskSource 提供最近一次关联任务，用于建议区展示失败原因与重试次数
// （*AiTaskService 满足）。
type RelationTaskSource interface {
	Latest(context.Context, uint, string, uint) (*model.AiTask, error)
	EnqueueRelationTask(context.Context, uint, uint, uint) (*model.AiTask, error)
}

type RelationService struct {
	store     RelationStore
	knowledge RelationKnowledgeStore
	tasks     RelationTaskSource
}

func NewRelationService(store RelationStore, knowledge RelationKnowledgeStore, tasks RelationTaskSource) *RelationService {
	return &RelationService{store: store, knowledge: knowledge, tasks: tasks}
}

// RelationCreateInput 是手动建立关系的请求体。
type RelationCreateInput struct {
	TargetID     uint               `json:"target_id"`
	RelationType model.RelationType `json:"relation_type"`
}

// RelationUpdateInput 是修改关系类型的请求体。
type RelationUpdateInput struct {
	RelationType model.RelationType `json:"relation_type"`
}

// RelationView 是关系接口视图：带对端知识，便于详情页直接跳转。
type RelationView struct {
	ID               uint                 `json:"id"`
	SourceID         uint                 `json:"source_id"`
	TargetID         uint                 `json:"target_id"`
	RelationType     model.RelationType   `json:"relation_type"`
	Status           model.RelationStatus `json:"status"`
	Source           model.RelationSource `json:"source"`
	Confidence       float64              `json:"confidence"`
	Evidence         string               `json:"evidence"`
	CounterpartID    uint                 `json:"counterpart_id"`
	CounterpartTitle string               `json:"counterpart_title"`
	ConfirmedAt      *time.Time           `json:"confirmed_at"`
	CreatedAt        time.Time            `json:"created_at"`
}

// RelationTaskView 是最近一次关联任务的状态，建议区用它提示失败原因与重试次数。
type RelationTaskView struct {
	ID           uint               `json:"id"`
	Status       model.AiTaskStatus `json:"status"`
	ErrorMessage string             `json:"error_message"`
	RetryCount   int                `json:"retry_count"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

// RelationList 按已确认/待确认分区返回，并附带最近一次关联任务。
type RelationList struct {
	Confirmed []RelationView    `json:"confirmed"`
	Suggested []RelationView    `json:"suggested"`
	Task      *RelationTaskView `json:"task"`
}

// Create 手动建立关系：直接落已确认，来源记为 manual。
func (s *RelationService) Create(ctx context.Context, userID, knowledgeID uint, input RelationCreateInput) (*RelationView, error) {
	relationType := model.RelationType(strings.TrimSpace(string(input.RelationType)))
	if !relationType.Valid() {
		return nil, fmt.Errorf("%w: 关系类型无效", ErrRelationInvalid)
	}
	if input.TargetID == 0 {
		return nil, fmt.Errorf("%w: 缺少目标知识", ErrRelationInvalid)
	}
	if input.TargetID == knowledgeID {
		return nil, fmt.Errorf("%w: 不能与自身建立关系", ErrRelationInvalid)
	}
	if _, err := s.knowledge.Get(ctx, userID, knowledgeID); err != nil {
		return nil, knowledgeError(err)
	}
	target, err := s.knowledge.Get(ctx, userID, input.TargetID)
	if err != nil {
		return nil, knowledgeError(err)
	}
	if target.ArchivedAt != nil {
		return nil, fmt.Errorf("%w: 目标知识已归档", ErrRelationInvalid)
	}
	now := time.Now()
	created, err := s.store.Create(ctx, &model.KnowledgeRelation{
		UserID:       userID,
		SourceID:     knowledgeID,
		TargetID:     input.TargetID,
		RelationType: relationType,
		Status:       model.RelationConfirmed,
		Source:       model.RelationSourceManual,
		ConfirmedAt:  &now,
	})
	if err != nil {
		return nil, err
	}
	view := relationViewFromRecord(repository.RelationRecord{KnowledgeRelation: *created})
	if created.SourceID != knowledgeID {
		// 对称类型入库时按 min/max 归一：当前知识成了 target，对端是 source。
		view.CounterpartID = created.SourceID
		view.CounterpartTitle = target.Title
	} else {
		view.CounterpartID = created.TargetID
		view.CounterpartTitle = target.Title
	}
	return &view, nil
}

// List 返回某知识的已确认关系、待确认建议与最近一次关联任务状态。
func (s *RelationService) List(ctx context.Context, userID, knowledgeID uint) (*RelationList, error) {
	if _, err := s.knowledge.Get(ctx, userID, knowledgeID); err != nil {
		return nil, knowledgeError(err)
	}
	rows, err := s.store.ListByKnowledge(ctx, userID, knowledgeID)
	if err != nil {
		return nil, err
	}
	list := &RelationList{
		Confirmed: make([]RelationView, 0, len(rows)),
		Suggested: make([]RelationView, 0, len(rows)),
	}
	for _, row := range rows {
		switch row.Status {
		case model.RelationConfirmed:
			list.Confirmed = append(list.Confirmed, relationViewFromRecord(row))
		case model.RelationSuggested:
			list.Suggested = append(list.Suggested, relationViewFromRecord(row))
		}
	}
	if s.tasks != nil {
		task, err := s.tasks.Latest(ctx, userID, model.AiTaskTypeRelation, knowledgeID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if err == nil {
			list.Task = relationTaskView(task)
		}
	}
	return list, nil
}

// Delete 物理删除关系；不属于当前用户时按不存在处理。
func (s *RelationService) Delete(ctx context.Context, userID, relationID uint) error {
	if err := s.store.Delete(ctx, userID, relationID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRelationNotFound
		}
		return err
	}
	return nil
}

// Confirm 确认一条 AI 建议：仅 suggested 可确认，同时记录确认时间。
// 已确认的关系再次确认保持幂等（返回 200 与当前状态）；已忽略的关系需手动添加恢复（409）。
func (s *RelationService) Confirm(ctx context.Context, userID, relationID uint) (*RelationView, error) {
	row, err := s.getRelation(ctx, userID, relationID)
	if err != nil {
		return nil, err
	}
	view := relationViewFromRecord(*row)
	switch row.Status {
	case model.RelationConfirmed:
		return &view, nil
	case model.RelationRejected:
		return nil, relationConflict("该建议已被忽略，可手动添加恢复")
	}
	now := time.Now()
	if err := s.store.UpdateStatus(ctx, userID, relationID, model.RelationConfirmed, &now); err != nil {
		return nil, relationMutationError(err)
	}
	view.Status = model.RelationConfirmed
	view.ConfirmedAt = &now
	return &view, nil
}

// Reject 忽略一条 AI 建议：置 rejected 并保留行，作为永久抑制标记。
// 已忽略再次忽略保持幂等；已确认的关系不能直接忽略，需先删除。
func (s *RelationService) Reject(ctx context.Context, userID, relationID uint) (*RelationView, error) {
	row, err := s.getRelation(ctx, userID, relationID)
	if err != nil {
		return nil, err
	}
	view := relationViewFromRecord(*row)
	switch row.Status {
	case model.RelationRejected:
		return &view, nil
	case model.RelationConfirmed:
		return nil, relationConflict("已确认的关系不能忽略，请删除后再试")
	}
	if err := s.store.UpdateStatus(ctx, userID, relationID, model.RelationRejected, nil); err != nil {
		return nil, relationMutationError(err)
	}
	view.Status = model.RelationRejected
	return &view, nil
}

// UpdateType 修改已确认关系的类型；归一后与既有同类型关系冲突时返回 409。
func (s *RelationService) UpdateType(ctx context.Context, userID, relationID uint, relationType model.RelationType) (*RelationView, error) {
	relationType = model.RelationType(strings.TrimSpace(string(relationType)))
	if !relationType.Valid() {
		return nil, fmt.Errorf("%w: 关系类型无效", ErrRelationInvalid)
	}
	row, err := s.getRelation(ctx, userID, relationID)
	if err != nil {
		return nil, err
	}
	if row.Status != model.RelationConfirmed {
		return nil, relationConflict("只有已确认的关系可以修改类型")
	}
	view := relationViewFromRecord(*row)
	if row.RelationType == relationType {
		return &view, nil
	}
	updated, err := s.store.UpdateType(ctx, userID, relationID, relationType)
	if err != nil {
		if errors.Is(err, repository.ErrRelationDuplicate) {
			return nil, relationConflict("该对知识已存在同类型关系")
		}
		return nil, relationMutationError(err)
	}
	view.RelationType = updated.RelationType
	view.SourceID = updated.SourceID
	view.TargetID = updated.TargetID
	return &view, nil
}

// Regenerate 手动触发一次关联推荐：活跃任务由仓储幂等复用，成功后再次触发会重新入队，
// 新建议由处理器与既有行合并（同一对跳过，不覆盖已确认关系）。
func (s *RelationService) Regenerate(ctx context.Context, userID, knowledgeID uint) (*RelationTaskView, error) {
	item, err := s.knowledge.Get(ctx, userID, knowledgeID)
	if err != nil {
		return nil, knowledgeError(err)
	}
	if item.ArchivedAt != nil {
		return nil, relationConflict("已归档的知识不再生成关联建议")
	}
	if s.tasks == nil {
		return nil, errors.New("relation task service unavailable")
	}
	task, err := s.tasks.EnqueueRelationTask(ctx, userID, knowledgeID, item.Version)
	if err != nil {
		return nil, err
	}
	return relationTaskView(task), nil
}

// getRelation 读取关系并把缺失映射为 404。
func (s *RelationService) getRelation(ctx context.Context, userID, relationID uint) (*repository.RelationRecord, error) {
	row, err := s.store.Get(ctx, userID, relationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRelationNotFound
		}
		return nil, err
	}
	return row, nil
}

// relationMutationError 把仓储层的缺失映射为 404，其余原样上抛。
func relationMutationError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrRelationNotFound
	}
	return err
}

func relationTaskView(task *model.AiTask) *RelationTaskView {
	if task == nil {
		return nil
	}
	return &RelationTaskView{
		ID: task.ID, Status: task.Status, ErrorMessage: task.ErrorMessage,
		RetryCount: task.RetryCount, UpdatedAt: task.UpdatedAt,
	}
}

func relationViewFromRecord(row repository.RelationRecord) RelationView {
	return RelationView{
		ID:               row.ID,
		SourceID:         row.SourceID,
		TargetID:         row.TargetID,
		RelationType:     row.RelationType,
		Status:           row.Status,
		Source:           row.Source,
		Confidence:       row.Confidence,
		Evidence:         row.Evidence,
		CounterpartID:    row.CounterpartID,
		CounterpartTitle: row.CounterpartTitle,
		ConfirmedAt:      row.ConfirmedAt,
		CreatedAt:        row.CreatedAt,
	}
}
