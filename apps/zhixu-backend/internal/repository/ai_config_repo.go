package repository

import (
	"context"
	"zhixu-backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AIConfigState struct {
	Providers []model.ModelProvider
	Models    []model.ProviderModel
	Settings  model.UserAISettings
}

type AIConfigRepository struct{ DB *gorm.DB }

func (r *AIConfigRepository) Read(ctx context.Context, user uint) (*AIConfigState, error) {
	var state *AIConfigState
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		state, err = readAIConfig(tx, user)
		return err
	})
	return state, err
}

func readAIConfig(db *gorm.DB, user uint) (*AIConfigState, error) {
	state := &AIConfigState{
		Providers: []model.ModelProvider{}, Models: []model.ProviderModel{},
		Settings: model.UserAISettings{UserID: user},
	}
	if err := db.Where("user_id = ?", user).Order("id ASC").Find(&state.Providers).Error; err != nil {
		return nil, err
	}
	if err := db.Where("user_id = ?", user).Order("id ASC").Find(&state.Models).Error; err != nil {
		return nil, err
	}
	return state, db.Where("user_id = ?", user).Limit(1).Find(&state.Settings).Error
}

// Serialize all configuration mutations on the existing user row, including first creation.
func (r *AIConfigRepository) Mutate(ctx context.Context, user uint, change func(*AIConfigState) error) (*AIConfigState, error) {
	var state *AIConfigState
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var owner model.UserModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").
			Where("id = ?", user).Take(&owner).Error; err != nil {
			return err
		}
		var err error
		state, err = readAIConfig(tx, user)
		if err != nil {
			return err
		}
		oldProviders := append([]model.ModelProvider(nil), state.Providers...)
		oldModels := append([]model.ProviderModel(nil), state.Models...)
		if err := change(state); err != nil {
			return err
		}
		for i := range state.Providers {
			provider := &state.Providers[i]
			provider.UserID = user
			if provider.ID == 0 {
				if err := tx.Create(provider).Error; err != nil {
					return err
				}
				// New models use ProviderID=0 until the provider insert allocates its ID.
				for j := range state.Models {
					if state.Models[j].ProviderID == 0 {
						state.Models[j].ProviderID = provider.ID
					}
				}
			} else if err := tx.Model(&model.ModelProvider{}).Where("id = ? AND user_id = ?", provider.ID, user).
				Select("name", "base_url", "protocol", "auth_type", "ciphertext").Updates(provider).Error; err != nil {
				return err
			}
		}
		for i := range state.Models {
			entry := &state.Models[i]
			entry.UserID = user
			if entry.ID == 0 {
				if err := tx.Create(entry).Error; err != nil {
					return err
				}
			} else if err := tx.Model(&model.ProviderModel{}).Where("id = ? AND user_id = ?", entry.ID, user).
				Select("kind", "dimension").Updates(entry).Error; err != nil {
				return err
			}
		}
		for _, old := range oldModels {
			found := false
			for _, current := range state.Models {
				if old.ID == current.ID {
					found = true
					break
				}
			}
			if !found {
				if err := tx.Where("id = ? AND user_id = ?", old.ID, user).Delete(&model.ProviderModel{}).Error; err != nil {
					return err
				}
			}
		}
		for _, old := range oldProviders {
			found := false
			for _, current := range state.Providers {
				if old.ID == current.ID {
					found = true
					break
				}
			}
			if !found {
				if err := tx.Where("id = ? AND user_id = ?", old.ID, user).Delete(&model.ModelProvider{}).Error; err != nil {
					return err
				}
			}
		}
		state.Settings.UserID = user
		state.Settings.ConfigVersion++
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"chat_model_id", "embedding_model_id", "rag_top_k", "rag_min_score", "config_version", "embedding_version", "needs_reindex", "updated_at"}),
		}).Create(&state.Settings).Error
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}
