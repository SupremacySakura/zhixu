package repository

import (
	"context"

	"zhixu-backend/internal/model"

	"gorm.io/gorm"
)

type RagRepository struct{ DB *gorm.DB }

func (r *RagRepository) Collections(ctx context.Context, user uint, version uint64) ([]string, error) {
	var names []string
	err := r.DB.WithContext(ctx).Model(&model.VectorizeTask{}).Where("user_id = ? AND embedding_version = ? AND collection <> ''", user, version).Distinct().Pluck("collection", &names).Error
	return names, err
}

func (r *RagRepository) ChunksByIDs(ctx context.Context, user uint, ids []uint) ([]model.KnowledgeChunk, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var chunks []model.KnowledgeChunk
	err := r.DB.WithContext(ctx).Where("user_id = ? AND id IN ? AND state = ? AND EXISTS (SELECT 1 FROM knowledge_items WHERE knowledge_items.id = knowledge_chunks.knowledge_id AND knowledge_items.user_id = knowledge_chunks.user_id AND knowledge_items.status = ?)", user, ids, "ready", model.KnowledgeReady).Find(&chunks).Error
	return chunks, err
}

func (r *RagRepository) KnowledgeTitles(ctx context.Context, user uint, ids []uint) (map[uint]string, error) {
	titles := map[uint]string{}
	if len(ids) == 0 {
		return titles, nil
	}
	var items []model.KnowledgeItem
	if err := r.DB.WithContext(ctx).Select("id", "title").Where("user_id = ? AND id IN ?", user, ids).Find(&items).Error; err != nil {
		return nil, err
	}
	for _, item := range items {
		titles[item.ID] = item.Title
	}
	return titles, nil
}
