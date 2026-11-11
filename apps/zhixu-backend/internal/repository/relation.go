package repository

import (
	"context"
	"errors"

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
// AI 推荐留下的 suggested/rejected 行会被提升为已确认并改为手动来源，不插入第二行。
// 并发插入命中唯一键时回读旧行，对上层表现为幂等。
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

func (r *RelationRepository) findPair(ctx context.Context, relation *model.KnowledgeRelation) (*model.KnowledgeRelation, error) {
	var row model.KnowledgeRelation
	err := r.DB.WithContext(ctx).
		Where("user_id = ? AND source_id = ? AND target_id = ? AND relation_type = ?",
			relation.UserID, relation.SourceID, relation.TargetID, relation.RelationType).
		Take(&row).Error
	return &row, err
}

// confirm 把旧行提升为已确认的手动关系；已是已确认行时原样返回。
func (r *RelationRepository) confirm(ctx context.Context, relation *model.KnowledgeRelation) (*model.KnowledgeRelation, error) {
	if relation.Status == model.RelationConfirmed {
		return relation, nil
	}
	if err := r.DB.WithContext(ctx).Model(&model.KnowledgeRelation{}).
		Where("id = ? AND user_id = ?", relation.ID, relation.UserID).
		Updates(map[string]any{
			"status": model.RelationConfirmed,
			"source": model.RelationSourceManual,
		}).Error; err != nil {
		return nil, err
	}
	relation.Status = model.RelationConfirmed
	relation.Source = model.RelationSourceManual
	return relation, nil
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
