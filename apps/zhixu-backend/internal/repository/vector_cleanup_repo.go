package repository

import (
	"context"
	"gorm.io/gorm"
	"time"
	"zhixu-backend/internal/model"
)

type VectorCleanupRepository struct{ DB *gorm.DB }

func (r *VectorCleanupRepository) Next(ctx context.Context) (*model.VectorCleanupTask, error) {
	var task model.VectorCleanupTask
	result := r.DB.WithContext(ctx).Where("next_attempt_at <= ?", time.Now()).Order("next_attempt_at ASC, id ASC").Limit(1).Find(&task)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &task, nil
}

// Keep durable tombstones even after success: a timed-out Qdrant write may
// complete remotely after the local transaction has rolled back.
func (r *VectorCleanupRepository) Reschedule(ctx context.Context, task *model.VectorCleanupTask, success bool) error {
	delay := 30 * time.Second
	if success {
		delay = 5 * time.Minute
	}
	return r.DB.WithContext(ctx).Model(&model.VectorCleanupTask{}).Where("id = ?", task.ID).Update("next_attempt_at", time.Now().Add(delay)).Error
}
