package service

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"time"
	"zhixu-backend/internal/log"
	"zhixu-backend/internal/model"
)

type VectorCleanupStore interface {
	Next(context.Context) (*model.VectorCleanupTask, error)
	Reschedule(context.Context, *model.VectorCleanupTask, bool) error
}
type VectorCleaner interface {
	UserCollections(context.Context, uint) ([]string, error)
	DeleteKnowledge(context.Context, string, uint, uint) error
}
type VectorCleanupService struct {
	Store   VectorCleanupStore
	Vectors VectorCleaner
}

func (s *VectorCleanupService) ProcessNext(ctx context.Context) (bool, error) {
	task, err := s.Store.Next(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	collections, err := s.Vectors.UserCollections(ctx, task.UserID)
	if err == nil {
		for _, collection := range collections {
			if err = s.Vectors.DeleteKnowledge(ctx, collection, task.UserID, task.KnowledgeID); err != nil {
				break
			}
		}
	}
	if retryErr := s.Store.Reschedule(ctx, task, err == nil); retryErr != nil {
		return true, retryErr
	}
	return true, err
}
func (s *VectorCleanupService) Run(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		for ctx.Err() == nil {
			processed, err := s.ProcessNext(ctx)
			if err != nil && ctx.Err() == nil {
				log.Error().Err(err).Msg("向量删除清理失败")
			}
			if !processed || err != nil {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
