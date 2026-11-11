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
)

// RelationStore 是关系仓储的最小能力集合（*repository.RelationRepository 满足）。
type RelationStore interface {
	Create(context.Context, *model.KnowledgeRelation) (*model.KnowledgeRelation, error)
	ListByKnowledge(context.Context, uint, uint) ([]repository.RelationRecord, error)
	Delete(context.Context, uint, uint) error
}

// RelationKnowledgeStore 用于校验关系两端的知识归属（*KnowledgeService 满足）。
type RelationKnowledgeStore interface {
	Get(context.Context, uint, uint) (*model.KnowledgeItem, error)
}

type RelationService struct {
	store     RelationStore
	knowledge RelationKnowledgeStore
}

func NewRelationService(store RelationStore, knowledge RelationKnowledgeStore) *RelationService {
	return &RelationService{store: store, knowledge: knowledge}
}

// RelationCreateInput 是手动建立关系的请求体。
type RelationCreateInput struct {
	TargetID     uint               `json:"target_id"`
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
	CreatedAt        time.Time            `json:"created_at"`
}

// RelationTaskView 为后续 AI 建议任务预留；本单不下发任务，恒为 nil。
type RelationTaskView struct {
	ID     uint               `json:"id"`
	Status model.AiTaskStatus `json:"status"`
}

// RelationList 按已确认/待确认分区返回；本单建议区恒为空。
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
	source, err := s.knowledge.Get(ctx, userID, knowledgeID)
	if err != nil {
		return nil, knowledgeError(err)
	}
	target, err := s.knowledge.Get(ctx, userID, input.TargetID)
	if err != nil {
		return nil, knowledgeError(err)
	}
	if target.ArchivedAt != nil {
		return nil, fmt.Errorf("%w: 目标知识已归档", ErrRelationInvalid)
	}
	created, err := s.store.Create(ctx, &model.KnowledgeRelation{
		UserID:       userID,
		SourceID:     knowledgeID,
		TargetID:     input.TargetID,
		RelationType: relationType,
		Status:       model.RelationConfirmed,
		Source:       model.RelationSourceManual,
	})
	if err != nil {
		return nil, err
	}
	view := relationViewFromRecord(repository.RelationRecord{KnowledgeRelation: *created})
	if created.SourceID != knowledgeID {
		// 对称类型入库时按 min/max 归一，对端要按归一后的方向取。
		view.CounterpartID = created.SourceID
		view.CounterpartTitle = source.Title
	} else {
		view.CounterpartID = created.TargetID
		view.CounterpartTitle = target.Title
	}
	return &view, nil
}

// List 返回某知识的已确认关系；建议区与任务状态留给后续批次。
func (s *RelationService) List(ctx context.Context, userID, knowledgeID uint) (*RelationList, error) {
	if _, err := s.knowledge.Get(ctx, userID, knowledgeID); err != nil {
		return nil, knowledgeError(err)
	}
	rows, err := s.store.ListByKnowledge(ctx, userID, knowledgeID)
	if err != nil {
		return nil, err
	}
	list := &RelationList{Confirmed: make([]RelationView, 0, len(rows)), Suggested: []RelationView{}}
	for _, row := range rows {
		if row.Status != model.RelationConfirmed {
			continue
		}
		list.Confirmed = append(list.Confirmed, relationViewFromRecord(row))
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
		CreatedAt:        row.CreatedAt,
	}
}
