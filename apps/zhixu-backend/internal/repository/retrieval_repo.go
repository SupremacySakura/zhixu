package repository

import (
	"context"

	"zhixu-backend/internal/model"

	"gorm.io/gorm"
)

// RetrievalLogRepository persists retrieval audit rows. Writes happen after the
// answer is ready, so a full experiment table never blocks the retrieval itself.
type RetrievalLogRepository struct {
	DB *gorm.DB
}

func NewRetrievalLogRepository(db *gorm.DB) *RetrievalLogRepository {
	return &RetrievalLogRepository{DB: db}
}

func (r *RetrievalLogRepository) Save(ctx context.Context, row *model.RetrievalLog) error {
	return r.DB.WithContext(ctx).Create(row).Error
}
