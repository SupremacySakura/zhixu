package repository

import (
	"context"
	"time"

	"zhixu-backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type VectorizeRepository struct{ DB *gorm.DB }

func EnqueueVectorization(tx *gorm.DB, user, knowledge uint) error {
	task := model.VectorizeTask{UserID: user, KnowledgeID: knowledge, Status: model.VectorPending}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&task).Error; err != nil {
		return err
	}
	return tx.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ? AND vector_state = ?", knowledge, user, "").
		Update("vector_state", model.VectorPending).Error
}

// Backfill only missing tasks, so failed paid calls are never retried on every restart.
func (r *VectorizeRepository) Backfill(ctx context.Context) error {
	var items []model.KnowledgeItem
	if err := r.DB.WithContext(ctx).Select("id", "user_id").Where(
		"status = ? AND NOT EXISTS (SELECT 1 FROM vectorize_tasks WHERE knowledge_id = knowledge_items.id)",
		model.KnowledgeReady).Limit(100).Find(&items).Error; err != nil {
		return err
	}
	for _, item := range items {
		if err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var current model.KnowledgeItem
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND user_id = ? AND status = ?", item.ID, item.UserID, model.KnowledgeReady).Take(&current).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return nil
				}
				return err
			}
			return EnqueueVectorization(tx, item.UserID, item.ID)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *VectorizeRepository) Claim(ctx context.Context, token string, now time.Time) (*model.VectorizeTask, error) {
	var task model.VectorizeTask
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("((status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)) OR (status = ? AND lease_expires_at < ?) OR (status = ? AND model_id = 0 AND collection = '' AND EXISTS (SELECT 1 FROM knowledge_items WHERE id = vectorize_tasks.knowledge_id AND vector_error = ?))) AND EXISTS (SELECT 1 FROM knowledge_items WHERE id = vectorize_tasks.knowledge_id AND user_id = vectorize_tasks.user_id AND status = ?)",
				model.VectorPending, now, model.VectorRunning, now, model.VectorPartial, "未配置可用的 Embedding 模型或凭据，请检查模型设置", model.KnowledgeReady).
			Order("id ASC").Limit(1).Find(&task)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		// Only this historical pre-call failure is safe to reschedule automatically.
		if task.Status == model.VectorPartial && task.ModelID == 0 && task.Collection == "" {
			if err := tx.Model(&model.KnowledgeChunk{}).Where("knowledge_id = ? AND user_id = ?", task.KnowledgeID, task.UserID).
				Updates(map[string]any{"state": "pending", "error_message": ""}).Error; err != nil {
				return err
			}
		}
		expires := now.Add(3 * time.Minute)
		if err := tx.Model(&task).Updates(map[string]any{
			"status": model.VectorRunning, "lease_token": token, "lease_expires_at": expires, "attempts": task.Attempts + 1,
		}).Error; err != nil {
			return err
		}
		task.Status, task.LeaseToken = model.VectorRunning, token
		return tx.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", task.KnowledgeID, task.UserID).
			Update("vector_state", model.VectorRunning).Error
	})
	return &task, err
}

// Every result transaction locks the task first, then the knowledge row, matching deletion.
func (r *VectorizeRepository) fenced(ctx context.Context, task *model.VectorizeTask, action func(*gorm.DB) error) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.VectorizeTask
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"id = ? AND user_id = ? AND status = ? AND lease_token = ? AND lease_expires_at > ?",
			task.ID, task.UserID, model.VectorRunning, task.LeaseToken, time.Now()).Limit(1).Find(&current).Error
		if err != nil {
			return err
		}
		if current.ID == 0 {
			return ErrKnowledgeConflict
		}
		var item model.KnowledgeItem
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", task.KnowledgeID, task.UserID).Limit(1).Find(&item)
		if result.Error != nil {
			return result.Error
		}
		if item.ID == 0 {
			return ErrKnowledgeConflict
		}
		if err := action(tx); err != nil {
			return err
		}
		return tx.Model(&model.VectorizeTask{}).Where("id = ?", task.ID).
			Update("lease_expires_at", time.Now().Add(3*time.Minute)).Error
	})
}

func (r *VectorizeRepository) Prepare(ctx context.Context, task *model.VectorizeTask, chunks []model.KnowledgeChunk, name string) error {
	return r.fenced(ctx, task, func(tx *gorm.DB) error {
		if err := tx.Model(&model.VectorizeTask{}).Where("id = ?", task.ID).Updates(map[string]any{
			"model_id": task.ModelID, "config_version": task.ConfigVersion,
			"embedding_version": task.EmbeddingVersion, "collection": task.Collection,
		}).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(chunks, 100).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.KnowledgeChunk{}).Where("knowledge_id = ? AND user_id = ? AND state = ?", task.KnowledgeID, task.UserID, "pending").Update("embedding_version", task.EmbeddingVersion).Error; err != nil {
			return err
		}
		return tx.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", task.KnowledgeID, task.UserID).
			Updates(map[string]any{"chunks_total": len(chunks), "embedding_model": name, "embedding_version": task.EmbeddingVersion, "vector_error": ""}).Error
	})
}

// WithLiveTask holds the same task -> knowledge locks used by deletion until
// Qdrant acknowledges the upsert. A tombstone can therefore never precede a late write.
func (r *VectorizeRepository) WithLiveTask(ctx context.Context, task *model.VectorizeTask, action func() error) error {
	return r.fenced(ctx, task, func(*gorm.DB) error { return action() })
}

func (r *VectorizeRepository) Wait(ctx context.Context, task *model.VectorizeTask, message string) error {
	return r.fenced(ctx, task, func(tx *gorm.DB) error {
		if err := tx.Model(&model.VectorizeTask{}).Where("id = ?", task.ID).Updates(map[string]any{
			"status": model.VectorPending, "lease_token": "", "next_attempt_at": time.Now().Add(30 * time.Second),
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", task.KnowledgeID, task.UserID).
			Updates(map[string]any{"vector_state": model.VectorPending, "vector_error": message}).Error
	})
}

func (r *VectorizeRepository) Renew(ctx context.Context, task *model.VectorizeTask) error {
	return r.fenced(ctx, task, func(*gorm.DB) error { return nil })
}

func (r *VectorizeRepository) SaveBatch(ctx context.Context, task *model.VectorizeTask, ids []uint, message string) error {
	return r.fenced(ctx, task, func(tx *gorm.DB) error {
		state := "ready"
		if message != "" {
			state = "failed"
		}
		if err := tx.Model(&model.KnowledgeChunk{}).
			Where("id IN ? AND user_id = ? AND knowledge_id = ?", ids, task.UserID, task.KnowledgeID).
			Updates(map[string]any{"state": state, "error_message": message}).Error; err != nil {
			return err
		}
		return updateVectorCounts(tx, task)
	})
}

func updateVectorCounts(tx *gorm.DB, task *model.VectorizeTask) error {
	var counts struct{ Ready, Failed int }
	if err := tx.Model(&model.KnowledgeChunk{}).Where("knowledge_id = ? AND user_id = ?", task.KnowledgeID, task.UserID).
		Select("COALESCE(SUM(state = 'ready'), 0) AS ready, COALESCE(SUM(state = 'failed'), 0) AS failed").Scan(&counts).Error; err != nil {
		return err
	}
	return tx.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", task.KnowledgeID, task.UserID).
		Updates(map[string]any{"chunks_ready": counts.Ready, "chunks_failed": counts.Failed}).Error
}

func (r *VectorizeRepository) Finish(ctx context.Context, task *model.VectorizeTask, message string) error {
	return r.fenced(ctx, task, func(tx *gorm.DB) error {
		state := model.VectorReady
		if message != "" {
			state = model.VectorPartial
			if err := tx.Model(&model.KnowledgeChunk{}).
				Where("knowledge_id = ? AND user_id = ? AND state = ?", task.KnowledgeID, task.UserID, "pending").
				Updates(map[string]any{"state": "failed", "error_message": message}).Error; err != nil {
				return err
			}
		}
		if err := updateVectorCounts(tx, task); err != nil {
			return err
		}
		if err := tx.Model(&model.VectorizeTask{}).Where("id = ?", task.ID).
			Updates(map[string]any{"status": state, "lease_token": ""}).Error; err != nil {
			return err
		}
		return tx.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", task.KnowledgeID, task.UserID).
			Updates(map[string]any{"vector_state": state, "vector_error": message}).Error
	})
}

func (r *VectorizeRepository) Chunks(ctx context.Context, user, id uint) ([]model.KnowledgeChunk, error) {
	items := make([]model.KnowledgeChunk, 0)
	err := r.DB.WithContext(ctx).Where("user_id = ? AND knowledge_id = ?", user, id).Order("seq ASC").Find(&items).Error
	return items, err
}
