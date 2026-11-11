package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"zhixu-backend/internal/model"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrKnowledgeConflict = errors.New("knowledge state conflict")

type KnowledgeRepository struct{ DB *gorm.DB }

func NewKnowledgeRepository(db *gorm.DB) *KnowledgeRepository { return &KnowledgeRepository{DB: db} }

// Delete tasks first to match the worker's lock order and fence in-flight results.
// 返回删除的知识条数，供调用方在级联删除日志中展示。
func DeleteKnowledgeRows(tx *gorm.DB, userID uint, query *gorm.DB) (int, error) {
	if err := tx.Where("user_id = ? AND knowledge_id IN (?)", userID, query.Select("id")).
		Delete(&model.KnowledgeImportTask{}).Error; err != nil {
		return 0, err
	}
	// Lock vector tasks before knowledge rows, matching fenced upserts. Capture
	// deletion tombstones before removing tasks; commit them atomically with deletion.
	var tasks []model.VectorizeTask
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND knowledge_id IN (?)", userID, query.Select("id")).Find(&tasks).Error; err != nil {
		return 0, err
	}
	var items []model.KnowledgeItem
	if err := query.Clauses(clause.Locking{Strength: "UPDATE"}).Find(&items).Error; err != nil {
		return 0, err
	}
	for _, item := range items {
		tombstone := model.VectorCleanupTask{UserID: userID, KnowledgeID: item.ID, NextAttemptAt: time.Now()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&tombstone).Error; err != nil {
			return 0, err
		}
	}
	for _, row := range []any{&model.VectorizeTask{}, &model.KnowledgeChunk{}, &model.KnowledgeTag{}} {
		if err := tx.Where("user_id = ? AND knowledge_id IN (?)", userID, query.Select("id")).Delete(row).Error; err != nil {
			return 0, err
		}
	}
	if err := query.Delete(&model.KnowledgeItem{}).Error; err != nil {
		return 0, err
	}
	return len(items), nil
}

func (r *KnowledgeRepository) Delete(ctx context.Context, userID, id uint) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Include user ownership in both deletes; do not require a live source file.
		query := tx.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", id, userID)
		_, err := DeleteKnowledgeRows(tx, userID, query)
		return err
	})
}

func (r *KnowledgeRepository) File(ctx context.Context, userID, fileID uint) (*model.FileModel, error) {
	var file model.FileModel
	err := r.DB.WithContext(ctx).Where("id = ? AND user_id = ?", fileID, userID).Take(&file).Error
	return &file, err
}

func (r *KnowledgeRepository) FolderName(ctx context.Context, userID, folderID uint) (string, error) {
	if folderID == 0 {
		return "根目录", nil
	}
	var folder model.FolderModel
	err := r.DB.WithContext(ctx).Where("id = ? AND user_id = ?", folderID, userID).Take(&folder).Error
	return folder.Name, err
}

func (r *KnowledgeRepository) ByFile(ctx context.Context, userID, fileID uint) (*model.KnowledgeItem, error) {
	var item model.KnowledgeItem
	err := r.DB.WithContext(ctx).Where("user_id = ? AND source_file_id = ?", userID, fileID).Take(&item).Error
	return &item, err
}

func (r *KnowledgeRepository) Get(ctx context.Context, userID, id uint) (*model.KnowledgeItem, error) {
	var item model.KnowledgeItem
	err := r.DB.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Take(&item).Error
	return &item, err
}

func (r *KnowledgeRepository) Create(ctx context.Context, item *model.KnowledgeItem) (*model.KnowledgeItem, bool, error) {
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var source model.FileModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", *item.SourceFileID, item.UserID).Take(&source).Error; err != nil {
			return err
		}
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		return tx.Create(&model.KnowledgeImportTask{KnowledgeID: item.ID, UserID: item.UserID, Status: model.KnowledgePending}).Error
	})
	if err == nil {
		return item, true, nil
	}
	var duplicate *mysqlDriver.MySQLError
	if errors.Is(err, gorm.ErrDuplicatedKey) || (errors.As(err, &duplicate) && duplicate.Number == 1062) {
		existing, lookupErr := r.ByFile(ctx, item.UserID, *item.SourceFileID)
		return existing, false, lookupErr
	}
	return nil, false, err
}

// CreateManual 落库手动创建的知识：正文已定稿，直接就绪并排入向量化队列，不走导入任务。
func (r *KnowledgeRepository) CreateManual(ctx context.Context, item *model.KnowledgeItem) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		return EnqueueVectorization(tx, item.UserID, item.ID)
	})
}

// UpdateMetadata 只更新 AI 整理产出的摘要与关键词：不触碰正文、版本号、分块与向量状态，
// 因此采纳整理结果不会让已有引用失效。先确认归属（0 行更新的 MySQL 语义无法区分"不存在"和"值未变"）。
func (r *KnowledgeRepository) UpdateMetadata(ctx context.Context, userID, id uint, summary, keywords string) error {
	db := r.DB.WithContext(ctx)
	var item model.KnowledgeItem
	if err := db.Select("id").Where("id = ? AND user_id = ?", id, userID).Take(&item).Error; err != nil {
		return err
	}
	return db.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{"summary": summary, "keywords": keywords}).Error
}

// SetFavorite 幂等设置收藏标记：值未变化时不写库。
func (r *KnowledgeRepository) SetFavorite(ctx context.Context, userID, id uint, favorite bool) error {
	db := r.DB.WithContext(ctx)
	var item model.KnowledgeItem
	if err := db.Select("id", "is_favorite").Where("id = ? AND user_id = ?", id, userID).Take(&item).Error; err != nil {
		return err
	}
	if item.IsFavorite == favorite {
		return nil
	}
	return db.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", id, userID).Update("is_favorite", favorite).Error
}

// SetArchived 归档（archived_at 置为当前时间）或恢复归档（清空），返回最新归档时间。
// 归档只影响列表与检索可见性，不删除分块与向量，恢复后立即重新生效。
func (r *KnowledgeRepository) SetArchived(ctx context.Context, userID, id uint, archived bool) (*time.Time, error) {
	db := r.DB.WithContext(ctx)
	var item model.KnowledgeItem
	if err := db.Select("id", "archived_at").Where("id = ? AND user_id = ?", id, userID).Take(&item).Error; err != nil {
		return nil, err
	}
	if !archived {
		if item.ArchivedAt == nil {
			return nil, nil
		}
		if err := db.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", id, userID).Update("archived_at", nil).Error; err != nil {
			return nil, err
		}
		return nil, nil
	}
	if item.ArchivedAt != nil {
		return item.ArchivedAt, nil
	}
	now := time.Now()
	if err := db.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", id, userID).Update("archived_at", now).Error; err != nil {
		return nil, err
	}
	return &now, nil
}

// Update 编辑知识正文：先保存编辑前的版本快照，再更新正文并重新排队分块与向量化。
// 锁序与向量化 worker 一致（先任务、后知识行）；旧分块的点 ID 记录到新任务，由任务启动时清理，
// 避免旧向量残留占用检索候选位。
func (r *KnowledgeRepository) Update(ctx context.Context, userID, id uint, title, content, sourceHash string, baseVersion uint) (*model.KnowledgeItem, error) {
	var updated model.KnowledgeItem
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.VectorizeTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND knowledge_id = ?", userID, id).Limit(1).Find(&task).Error; err != nil {
			return err
		}
		var item model.KnowledgeItem
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", id, userID).Limit(1).Find(&item).Error; err != nil {
			return err
		}
		if item.ID == 0 {
			return gorm.ErrRecordNotFound
		}
		if item.Status != model.KnowledgeReady || item.Version != baseVersion {
			return ErrKnowledgeConflict
		}
		updated = item
		// 未改动标题与正文时不产生新版本，也不重复调用 Embedding。
		if item.Title == title && item.Content == content {
			return nil
		}
		if err := tx.Create(&model.KnowledgeVersion{
			UserID: userID, KnowledgeID: id, VersionNumber: item.Version,
			Title: item.Title, Content: item.Content, ChangeSource: model.KnowledgeChangeSourceManual,
		}).Error; err != nil {
			return err
		}
		var staleIDs []uint
		if err := tx.Model(&model.KnowledgeChunk{}).Where("user_id = ? AND knowledge_id = ?", userID, id).
			Pluck("id", &staleIDs).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", id, userID).Updates(map[string]any{
			"title": title, "content": content, "source_hash": sourceHash, "version": item.Version + 1,
			"vector_state": "", "chunks_total": 0, "chunks_ready": 0, "chunks_failed": 0,
			"embedding_model": "", "embedding_version": 0, "vector_error": "",
		}).Error; err != nil {
			return err
		}
		for _, row := range []any{&model.VectorizeTask{}, &model.KnowledgeChunk{}} {
			if err := tx.Where("user_id = ? AND knowledge_id = ?", userID, id).Delete(row).Error; err != nil {
				return err
			}
		}
		if err := EnqueueVectorization(tx, userID, id); err != nil {
			return err
		}
		if task.Collection != "" && len(staleIDs) > 0 {
			payload, err := json.Marshal(staleIDs)
			if err != nil {
				return err
			}
			if err := tx.Model(&model.VectorizeTask{}).Where("user_id = ? AND knowledge_id = ?", userID, id).
				Updates(map[string]any{"stale_chunk_ids": string(payload), "cleanup_collection": task.Collection}).Error; err != nil {
				return err
			}
		}
		updated.Title, updated.Content, updated.SourceHash = title, content, sourceHash
		updated.Version = item.Version + 1
		updated.VectorState = model.VectorPending
		updated.ChunksTotal, updated.ChunksReady, updated.ChunksFailed = 0, 0, 0
		updated.EmbeddingModel, updated.EmbeddingVersion, updated.VectorError = "", 0, ""
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

// ListVersions 返回版本快照摘要（不含正文），按版本号倒序；当前版本由服务层用条目实时合成。
func (r *KnowledgeRepository) ListVersions(ctx context.Context, userID, knowledgeID uint) ([]model.KnowledgeVersion, error) {
	versions := make([]model.KnowledgeVersion, 0)
	err := r.DB.WithContext(ctx).Omit("content").
		Where("user_id = ? AND knowledge_id = ?", userID, knowledgeID).
		Order("version_number DESC").Find(&versions).Error
	return versions, err
}

// GetVersion 返回指定版本快照（含正文）。
func (r *KnowledgeRepository) GetVersion(ctx context.Context, userID, knowledgeID, versionNumber uint) (*model.KnowledgeVersion, error) {
	var version model.KnowledgeVersion
	err := r.DB.WithContext(ctx).
		Where("user_id = ? AND knowledge_id = ? AND version_number = ?", userID, knowledgeID, versionNumber).
		Take(&version).Error
	return &version, err
}

// KnowledgeListFilter 汇总知识列表的筛选、排序与分页条件。
type KnowledgeListFilter struct {
	Page        int
	Size        int
	Query       string
	Status      model.KnowledgeStatus
	VectorState model.VectorState
	SourceType  string
	TagID       uint
	Favorite    *bool
	Archived    bool
	Sort        string
}

func (r *KnowledgeRepository) List(ctx context.Context, userID uint, page, size int, query string, status model.KnowledgeStatus) ([]model.KnowledgeItem, int64, error) {
	return r.ListVector(ctx, userID, KnowledgeListFilter{Page: page, Size: size, Query: query, Status: status})
}

func (r *KnowledgeRepository) ListVector(ctx context.Context, userID uint, filter KnowledgeListFilter) ([]model.KnowledgeItem, int64, error) {
	// 归档知识默认不出现在列表中；检索侧另有 archived_at 过滤保证问答不可见。
	db := r.DB.WithContext(ctx).Model(&model.KnowledgeItem{}).Where("user_id = ?", userID)
	if filter.Archived {
		db = db.Where("archived_at IS NOT NULL")
	} else {
		db = db.Where("archived_at IS NULL")
	}
	if filter.Query != "" {
		db = db.Where("LOCATE(?, title) > 0", filter.Query)
	}
	if filter.Status != "" {
		db = db.Where("status = ?", filter.Status)
	}
	if filter.VectorState != "" {
		db = db.Where("vector_state = ?", filter.VectorState)
	}
	if filter.SourceType != "" {
		db = db.Where("source_type = ?", filter.SourceType)
	}
	if filter.TagID > 0 {
		db = db.Joins("JOIN knowledge_tags ON knowledge_tags.knowledge_id = knowledge_items.id AND knowledge_tags.user_id = knowledge_items.user_id").
			Where("knowledge_tags.tag_id = ?", filter.TagID)
	}
	if filter.Favorite != nil {
		db = db.Where("is_favorite = ?", *filter.Favorite)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := "created_at DESC, id DESC"
	switch filter.Sort {
	case "updated":
		order = "updated_at DESC, id DESC"
	case "title":
		order = "title ASC, id ASC"
	}
	items := make([]model.KnowledgeItem, 0)
	err := db.Omit("content", "source_hash").Order(order).Offset((filter.Page - 1) * filter.Size).Limit(filter.Size).Find(&items).Error
	return items, total, err
}

func (r *KnowledgeRepository) Retry(ctx context.Context, userID, id uint) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock task before item, matching the worker lock order.
		var task model.KnowledgeImportTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("knowledge_id = ? AND user_id = ?", id, userID).Take(&task).Error; err != nil {
			return err
		}
		if task.Status != model.KnowledgeFailed {
			return ErrKnowledgeConflict
		}
		if err := tx.Model(&task).Updates(map[string]any{"status": model.KnowledgePending, "lease_token": "", "lease_expires_at": nil}).Error; err != nil {
			return err
		}
		return tx.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", id, userID).Updates(map[string]any{"status": model.KnowledgePending, "error_code": "", "error_message": ""}).Error
	})
}

func (r *KnowledgeRepository) Claim(ctx context.Context, token string, now time.Time) (*model.KnowledgeImportTask, error) {
	var task model.KnowledgeImportTask
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? OR (status = ? AND (lease_expires_at IS NULL OR lease_expires_at < ?))", model.KnowledgePending, model.KnowledgeRunning, now).
			Order("id ASC").Limit(1).Find(&task)
		if result.Error != nil {
			return result.Error
		}
		// An idle queue is expected; Find avoids GORM logging it as an error.
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		expires := now.Add(2 * time.Minute)
		if err := tx.Model(&task).Updates(map[string]any{"status": model.KnowledgeRunning, "lease_token": token, "lease_expires_at": expires, "attempts": task.Attempts + 1}).Error; err != nil {
			return err
		}
		task.LeaseToken, task.Status = token, model.KnowledgeRunning
		return tx.Model(&model.KnowledgeItem{}).Where("id = ?", task.KnowledgeID).Update("status", model.KnowledgeRunning).Error
	})
	return &task, err
}

func (r *KnowledgeRepository) Finish(ctx context.Context, task *model.KnowledgeImportTask, content, code, message string) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		status := model.KnowledgeReady
		if code != "" {
			status = model.KnowledgeFailed
		}
		result := tx.Model(&model.KnowledgeImportTask{}).
			Where("id = ? AND status = ? AND lease_token = ?", task.ID, model.KnowledgeRunning, task.LeaseToken).
			Updates(map[string]any{"status": status, "lease_token": "", "lease_expires_at": nil})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrKnowledgeConflict
		}
		values := map[string]any{"status": status, "error_code": code, "error_message": message}
		if code == "" {
			values["content"], values["imported_at"] = content, time.Now()
		}
		if err := tx.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", task.KnowledgeID, task.UserID).Updates(values).Error; err != nil {
			return err
		}
		if code == "" {
			return EnqueueVectorization(tx, task.UserID, task.KnowledgeID)
		}
		return nil
	})
}
