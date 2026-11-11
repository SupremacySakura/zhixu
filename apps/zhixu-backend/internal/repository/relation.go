package repository

import (
	"context"
	"errors"
	"time"

	"zhixu-backend/internal/model"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// RelationRecord 一条关系及其对端知识（详情页展示对端标题与跳转用）。
type RelationRecord struct {
	model.KnowledgeRelation
	CounterpartID    uint   `gorm:"column:counterpart_id" json:"counterpart_id"`
	CounterpartTitle string `gorm:"column:counterpart_title" json:"counterpart_title"`
}

// ErrRelationDuplicate 表示修改类型后命中了唯一键：同一对知识已存在同类型关系。
var ErrRelationDuplicate = errors.New("relation duplicate")

type RelationRepository struct{ DB *gorm.DB }

func NewRelationRepository(db *gorm.DB) *RelationRepository { return &RelationRepository{DB: db} }

// normalizeRelationPair 对称关系按两侧知识 ID 的 min/max 归一，保证同一对只有一行；
// 有向关系保持入参方向。
func normalizeRelationPair(relationType model.RelationType, sourceID, targetID uint) (uint, uint) {
	if relationType.Symmetric() && sourceID > targetID {
		return targetID, sourceID
	}
	return sourceID, targetID
}

// Create 写入关系并返回落库后的行。同一对知识同类型已存在时复用旧行：
// suggested 行只提升状态、保留 AI 来源（用户只是确认建议）；rejected 行被手动添加
// 命中时改为 confirmed + manual（覆盖用户此前的忽略决定）。并发插入命中唯一键时
// 回读旧行，对上层表现为幂等。
func (r *RelationRepository) Create(ctx context.Context, relation *model.KnowledgeRelation) (*model.KnowledgeRelation, error) {
	relation.SourceID, relation.TargetID = normalizeRelationPair(relation.RelationType, relation.SourceID, relation.TargetID)
	existing, err := r.findPair(ctx, relation)
	if err == nil {
		return r.confirm(ctx, existing)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err := r.DB.WithContext(ctx).Create(relation).Error; err != nil {
		if !isDuplicateRelation(err) {
			return nil, err
		}
		existing, err = r.findPair(ctx, relation)
		if err != nil {
			return nil, err
		}
		return r.confirm(ctx, existing)
	}
	return relation, nil
}

// CreateSuggestion 写入 AI 建议：同一对知识同类型只要已有任意状态的关系（含已拒绝）
// 就不再插入，既不覆盖用户已确认的行，也不重复堆积建议；不同类型可以共存。
// 并发命中唯一键时同样视为已存在。
func (r *RelationRepository) CreateSuggestion(ctx context.Context, relation *model.KnowledgeRelation) (bool, error) {
	relation.SourceID, relation.TargetID = normalizeRelationPair(relation.RelationType, relation.SourceID, relation.TargetID)
	var count int64
	if err := r.DB.WithContext(ctx).Model(&model.KnowledgeRelation{}).
		Where("user_id = ? AND source_id = ? AND target_id = ? AND relation_type = ?",
			relation.UserID, relation.SourceID, relation.TargetID, relation.RelationType).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	if err := r.DB.WithContext(ctx).Create(relation).Error; err != nil {
		if isDuplicateRelation(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *RelationRepository) findPair(ctx context.Context, relation *model.KnowledgeRelation) (*model.KnowledgeRelation, error) {
	var row model.KnowledgeRelation
	err := r.DB.WithContext(ctx).
		Where("user_id = ? AND source_id = ? AND target_id = ? AND relation_type = ?",
			relation.UserID, relation.SourceID, relation.TargetID, relation.RelationType).
		Take(&row).Error
	return &row, err
}

// confirm 把旧行提升为已确认关系；已是已确认行时原样返回。
// 只有被手动添加命中的 rejected 行才改为 manual：确认一条 AI 建议不应篡改来源。
func (r *RelationRepository) confirm(ctx context.Context, relation *model.KnowledgeRelation) (*model.KnowledgeRelation, error) {
	if relation.Status == model.RelationConfirmed {
		return relation, nil
	}
	confirmedAt := time.Now()
	wasRejected := relation.Status == model.RelationRejected
	updates := map[string]any{
		"status":       model.RelationConfirmed,
		"confirmed_at": confirmedAt,
	}
	if wasRejected {
		updates["source"] = model.RelationSourceManual
	}
	if err := r.DB.WithContext(ctx).Model(&model.KnowledgeRelation{}).
		Where("id = ? AND user_id = ?", relation.ID, relation.UserID).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	relation.Status = model.RelationConfirmed
	if wasRejected {
		relation.Source = model.RelationSourceManual
	}
	relation.ConfirmedAt = &confirmedAt
	return relation, nil
}

// Get 按关系 ID 读取归属当前用户的单行；不存在或属于他人时返回 gorm.ErrRecordNotFound。
func (r *RelationRepository) Get(ctx context.Context, userID, id uint) (*RelationRecord, error) {
	var row RelationRecord
	err := r.DB.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Take(&row).Error
	return &row, err
}

// UpdateStatus 置关系状态；confirmedAt 非空时同时记录确认时间（忽略不写）。
func (r *RelationRepository) UpdateStatus(ctx context.Context, userID, id uint, status model.RelationStatus, confirmedAt *time.Time) error {
	updates := map[string]any{"status": status}
	if confirmedAt != nil {
		updates["confirmed_at"] = *confirmedAt
	}
	res := r.DB.WithContext(ctx).Model(&model.KnowledgeRelation{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateType 修改已确认关系的类型：对称类型按 min/max 归一后写回，
// 归一后命中唯一键时返回 ErrRelationDuplicate，供上层映射为 409。
func (r *RelationRepository) UpdateType(ctx context.Context, userID, id uint, relationType model.RelationType) (*model.KnowledgeRelation, error) {
	var row model.KnowledgeRelation
	if err := r.DB.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Take(&row).Error; err != nil {
		return nil, err
	}
	sourceID, targetID := normalizeRelationPair(relationType, row.SourceID, row.TargetID)
	err := r.DB.WithContext(ctx).Model(&model.KnowledgeRelation{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{
			"relation_type": relationType,
			"source_id":     sourceID,
			"target_id":     targetID,
		}).Error
	if err != nil {
		if isDuplicateRelation(err) {
			return nil, ErrRelationDuplicate
		}
		return nil, err
	}
	row.RelationType = relationType
	row.SourceID = sourceID
	row.TargetID = targetID
	return &row, nil
}

// ListByKnowledge 返回某条知识两个方向的关系，按创建时间倒序；对端标题随行返回。
func (r *RelationRepository) ListByKnowledge(ctx context.Context, userID, knowledgeID uint) ([]RelationRecord, error) {
	var rows []RelationRecord
	err := r.DB.WithContext(ctx).
		Table("knowledge_relations AS r").
		Select("r.*, k.id AS counterpart_id, k.title AS counterpart_title").
		Joins("JOIN knowledge_items k ON k.id = IF(r.source_id = ?, r.target_id, r.source_id)", knowledgeID).
		Where("r.user_id = ? AND (r.source_id = ? OR r.target_id = ?)", userID, knowledgeID, knowledgeID).
		Order("r.created_at DESC, r.id DESC").
		Scan(&rows).Error
	return rows, err
}

// Delete 物理删除关系；不存在或不属于该用户时返回 gorm.ErrRecordNotFound。
func (r *RelationRepository) Delete(ctx context.Context, userID, id uint) error {
	res := r.DB.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.KnowledgeRelation{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// isDuplicateRelation 判断数据库唯一键冲突。
func isDuplicateRelation(err error) bool {
	var duplicate *mysqlDriver.MySQLError
	return errors.Is(err, gorm.ErrDuplicatedKey) || (errors.As(err, &duplicate) && duplicate.Number == 1062)
}
