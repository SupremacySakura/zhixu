package repository

import (
	"context"
	"strings"

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
	// 归档知识不参与问答检索：状态就绪且未归档的条目才会返回候选分块。
	err := r.DB.WithContext(ctx).Where("user_id = ? AND id IN ? AND state = ? AND EXISTS (SELECT 1 FROM knowledge_items WHERE knowledge_items.id = knowledge_chunks.knowledge_id AND knowledge_items.user_id = knowledge_chunks.user_id AND knowledge_items.status = ? AND knowledge_items.archived_at IS NULL)", user, ids, "ready", model.KnowledgeReady).Find(&chunks).Error
	return chunks, err
}

func (r *RagRepository) KnowledgeTitles(ctx context.Context, user uint, ids []uint) (map[uint]string, error) {
	titles := map[uint]string{}
	if len(ids) == 0 {
		return titles, nil
	}
	var items []model.KnowledgeItem
	if err := r.DB.WithContext(ctx).Select("id", "title").Where("user_id = ? AND id IN ? AND archived_at IS NULL", user, ids).Find(&items).Error; err != nil {
		return nil, err
	}
	for _, item := range items {
		titles[item.ID] = item.Title
	}
	return titles, nil
}

// SearchChunksKeyword matches the query against the caller's live chunks through
// the ngram full-text index, strongest relevance first. The joins keep archived
// or non-ready knowledge out of the candidate set, mirroring the semantic path.
func (r *RagRepository) SearchChunksKeyword(ctx context.Context, user uint, query string, limit int) ([]model.KnowledgeChunk, error) {
	query = strings.TrimSpace(query)
	if query == "" || limit <= 0 {
		return nil, nil
	}
	var chunks []model.KnowledgeChunk
	err := r.DB.WithContext(ctx).Raw(`
		SELECT knowledge_chunks.*
		FROM knowledge_chunks
		JOIN knowledge_items
			ON knowledge_items.id = knowledge_chunks.knowledge_id
			AND knowledge_items.user_id = knowledge_chunks.user_id
		WHERE knowledge_chunks.user_id = ?
			AND knowledge_chunks.state = ?
			AND knowledge_items.status = ?
			AND knowledge_items.archived_at IS NULL
			AND MATCH(knowledge_chunks.content) AGAINST(? IN NATURAL LANGUAGE MODE)
		ORDER BY MATCH(knowledge_chunks.content) AGAINST(? IN NATURAL LANGUAGE MODE) DESC, knowledge_chunks.id ASC
		LIMIT ?`, user, "ready", model.KnowledgeReady, query, query, limit).Scan(&chunks).Error
	return chunks, err
}
