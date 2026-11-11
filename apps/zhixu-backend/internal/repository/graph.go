package repository

import (
	"context"
	"time"

	"zhixu-backend/internal/model"

	"gorm.io/gorm"
)

// GraphNodeRow 是图谱渲染所需的最小知识字段；正文不进入图谱查询。
type GraphNodeRow struct {
	ID         uint      `gorm:"column:id"`
	Title      string    `gorm:"column:title"`
	SourceType string    `gorm:"column:source_type"`
	Summary    string    `gorm:"column:summary"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

const graphNodeColumns = "id, title, source_type, summary, updated_at"

// GraphNodeFilter 描述图谱节点集合的筛选条件；零值表示不筛选。
// 标签通过 knowledge_tags 连接命中，来源类型精确匹配，两者同时给出时取交集。
type GraphNodeFilter struct {
	TagID      uint
	SourceType string
}

func (f GraphNodeFilter) apply(query *gorm.DB) *gorm.DB {
	if f.TagID > 0 {
		query = query.
			Joins("JOIN knowledge_tags ON knowledge_tags.knowledge_id = knowledge_items.id AND knowledge_tags.user_id = knowledge_items.user_id").
			Where("knowledge_tags.tag_id = ?", f.TagID)
	}
	if f.SourceType != "" {
		query = query.Where("source_type = ?", f.SourceType)
	}
	return query
}

// GraphRepository 提供局部图谱的节点与边读取。
// 所有节点查询都限定「当前用户 + 未归档」，归档与归属过滤在 SQL 层完成。
type GraphRepository struct{ DB *gorm.DB }

func NewGraphRepository(db *gorm.DB) *GraphRepository { return &GraphRepository{DB: db} }

// GetNode 读取一个未归档的知识；不存在、已归档或属于他人时返回 gorm.ErrRecordNotFound。
func (r *GraphRepository) GetNode(ctx context.Context, userID, id uint) (*GraphNodeRow, error) {
	var row GraphNodeRow
	err := r.DB.WithContext(ctx).
		Table("knowledge_items").
		Select(graphNodeColumns).
		Where("id = ? AND user_id = ? AND archived_at IS NULL", id, userID).
		Take(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ListRecentNodes 返回最近更新的未归档知识，用于无中心时的默认子图。
func (r *GraphRepository) ListRecentNodes(ctx context.Context, userID uint, filter GraphNodeFilter, limit int) ([]GraphNodeRow, error) {
	rows := make([]GraphNodeRow, 0)
	query := filter.apply(r.DB.WithContext(ctx).
		Table("knowledge_items").
		Select(graphNodeColumns).
		Where("knowledge_items.user_id = ? AND knowledge_items.archived_at IS NULL", userID))
	err := query.
		Order("updated_at DESC, id DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// ListNodesByIDs 批量读取节点，避免逐节点查询；归档与归属过滤仍在 SQL 层。
func (r *GraphRepository) ListNodesByIDs(ctx context.Context, userID uint, filter GraphNodeFilter, ids []uint) ([]GraphNodeRow, error) {
	rows := make([]GraphNodeRow, 0)
	if len(ids) == 0 {
		return rows, nil
	}
	query := filter.apply(r.DB.WithContext(ctx).
		Table("knowledge_items").
		Select(graphNodeColumns).
		Where("knowledge_items.user_id = ? AND knowledge_items.archived_at IS NULL AND knowledge_items.id IN ?", userID, ids))
	err := query.
		Order("id ASC").
		Find(&rows).Error
	return rows, err
}

// ListRelations 一次读取与给定节点相关的全部关系（任一端命中），
// 上层再按最终节点集裁剪；默认只有 confirmed，includeSuggested 时追加 suggested。
func (r *GraphRepository) ListRelations(ctx context.Context, userID uint, ids []uint, includeSuggested bool) ([]model.KnowledgeRelation, error) {
	rows := make([]model.KnowledgeRelation, 0)
	if len(ids) == 0 {
		return rows, nil
	}
	statuses := []model.RelationStatus{model.RelationConfirmed}
	if includeSuggested {
		statuses = append(statuses, model.RelationSuggested)
	}
	query := r.DB.WithContext(ctx).Where("user_id = ? AND status IN ?", userID, statuses)
	query = query.Where(r.DB.Where("source_id IN ?", ids).Or("target_id IN ?", ids))
	err := query.Order("id ASC").Find(&rows).Error
	return rows, err
}

// ListRelationsAmong 读取两端都在给定节点集合内的关系，
// 供筛选后的子图使用；默认只有 confirmed，includeSuggested 时追加 suggested。
func (r *GraphRepository) ListRelationsAmong(ctx context.Context, userID uint, ids []uint, includeSuggested bool) ([]model.KnowledgeRelation, error) {
	rows := make([]model.KnowledgeRelation, 0)
	if len(ids) == 0 {
		return rows, nil
	}
	statuses := []model.RelationStatus{model.RelationConfirmed}
	if includeSuggested {
		statuses = append(statuses, model.RelationSuggested)
	}
	err := r.DB.WithContext(ctx).
		Where("user_id = ? AND status IN ?", userID, statuses).
		Where("source_id IN ? AND target_id IN ?", ids, ids).
		Order("id ASC").
		Find(&rows).Error
	return rows, err
}
