package repository

import (
	"context"
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
func DeleteKnowledgeRows(tx *gorm.DB, userID uint, query *gorm.DB) error {
	if err := tx.Where("user_id = ? AND knowledge_id IN (?)", userID, query.Select("id")).
		Delete(&model.KnowledgeImportTask{}).Error; err != nil {
		return err
	}
	// Lock vector tasks before knowledge rows, matching fenced upserts. Capture
	// deletion tombstones before removing tasks; commit them atomically with deletion.
	var tasks []model.VectorizeTask
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND knowledge_id IN (?)", userID, query.Select("id")).Find(&tasks).Error; err != nil {
		return err
	}
	var items []model.KnowledgeItem
	if err := query.Clauses(clause.Locking{Strength: "UPDATE"}).Find(&items).Error; err != nil {
		return err
	}
	for _, item := range items {
		tombstone := model.VectorCleanupTask{UserID: userID, KnowledgeID: item.ID, NextAttemptAt: time.Now()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&tombstone).Error; err != nil {
			return err
		}
	}
	for _, row := range []any{&model.VectorizeTask{}, &model.KnowledgeChunk{}} {
		if err := tx.Where("user_id = ? AND knowledge_id IN (?)", userID, query.Select("id")).Delete(row).Error; err != nil {
			return err
		}
	}
	return query.Delete(&model.KnowledgeItem{}).Error
}

func (r *KnowledgeRepository) Delete(ctx context.Context, userID, id uint) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Include user ownership in both deletes; do not require a live source file.
		query := tx.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", id, userID)
		return DeleteKnowledgeRows(tx, userID, query)
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
			Where("id = ? AND user_id = ?", item.SourceFileID, item.UserID).Take(&source).Error; err != nil {
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
		existing, lookupErr := r.ByFile(ctx, item.UserID, item.SourceFileID)
		return existing, false, lookupErr
	}
	return nil, false, err
}

func (r *KnowledgeRepository) List(ctx context.Context, userID uint, page, size int, query string, status model.KnowledgeStatus) ([]model.KnowledgeItem, int64, error) {
	return r.ListVector(ctx, userID, page, size, query, status, "")
}

func (r *KnowledgeRepository) ListVector(ctx context.Context, userID uint, page, size int, query string, status model.KnowledgeStatus, vector model.VectorState) ([]model.KnowledgeItem, int64, error) {
	db := r.DB.WithContext(ctx).Model(&model.KnowledgeItem{}).Where("user_id = ?", userID)
	if query != "" {
		db = db.Where("LOCATE(?, title) > 0", query)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if vector != "" {
		db = db.Where("vector_state = ?", vector)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.KnowledgeItem, 0)
	err := db.Omit("content", "source_hash").Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error
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
