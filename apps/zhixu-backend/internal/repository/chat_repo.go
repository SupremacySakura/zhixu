package repository

import (
	"context"
	"strings"

	"zhixu-backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ChatSourceRepository stores provenance and durable deletion state in MySQL.
type ChatSourceRepository struct {
	DB *gorm.DB
}

func NewChatSourceRepository(db *gorm.DB) *ChatSourceRepository {
	return &ChatSourceRepository{DB: db}
}

// lockConversation creates a persistent serialization point even when no sources
// exist yet. The composite primary key's upsert locks the existing row; FOR UPDATE
// reads its current tombstone under the same transaction. No process-local mutex is
// relied on, and duplicate creation must never overwrite a tombstone.
func lockConversation(tx *gorm.DB, conversationID string, user uint) (*model.ChatConversationState, error) {
	state := &model.ChatConversationState{UserID: user, ConversationID: conversationID}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(state).Error; err != nil {
		return nil, err
	}
	state = &model.ChatConversationState{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND conversation_id = ?", user, conversationID).Take(state).Error; err != nil {
		return nil, err
	}
	return state, nil
}

func (r *ChatSourceRepository) SaveTurnSource(ctx context.Context, row *model.ChatTurnSource) error {
	if row == nil || strings.TrimSpace(row.MessageID) == "" || row.ConversationID == "" {
		return nil // Cannot associate an unfinished/legacy turn safely.
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state, err := lockConversation(tx, row.ConversationID, row.UserID)
		if err != nil {
			return err
		}
		if state.Tombstoned {
			return model.ErrConversationDeleted
		}
		return tx.Create(row).Error
	})
}

func (r *ChatSourceRepository) ListTurnSources(ctx context.Context, conversationID string, user uint) ([]model.ChatTurnSource, error) {
	var rows []model.ChatTurnSource
	err := r.DB.WithContext(ctx).
		Where("conversation_id = ? AND user_id = ?", conversationID, user).
		Order("id ASC").
		Find(&rows).Error
	return rows, err
}

func (r *ChatSourceRepository) IsDeleted(ctx context.Context, conversationID string, user uint) (bool, error) {
	var state model.ChatConversationState
	err := r.DB.WithContext(ctx).Where("user_id = ? AND conversation_id = ?", user, conversationID).Take(&state).Error
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	return state.Tombstoned, err
}

// MarkDeleted commits the tombstone and source removal BEFORE any remote deletion.
// Every deletion attempt is pending until remote deletion and local completion succeed.
func (r *ChatSourceRepository) MarkDeleted(ctx context.Context, conversationID string, user uint) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state, err := lockConversation(tx, conversationID, user)
		if err != nil {
			return err
		}
		if err := tx.Model(state).Updates(map[string]any{"tombstoned": true, "cleanup_pending": true}).Error; err != nil {
			return err
		}
		return tx.Where("conversation_id = ? AND user_id = ?", conversationID, user).Delete(&model.ChatTurnSource{}).Error
	})
}

func (r *ChatSourceRepository) CompleteCleanup(ctx context.Context, conversationID string, user uint) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state, err := lockConversation(tx, conversationID, user)
		if err != nil {
			return err
		}
		if !state.Tombstoned {
			return nil
		}
		if err := tx.Where("conversation_id = ? AND user_id = ?", conversationID, user).Delete(&model.ChatTurnSource{}).Error; err != nil {
			return err
		}
		// Never delete the state row or unset tombstoned: late streams stay fenced.
		return tx.Model(state).Update("cleanup_pending", false).Error
	})
}

func (r *ChatSourceRepository) PendingCleanups(ctx context.Context) ([]model.ChatConversationState, error) {
	var rows []model.ChatConversationState
	err := r.DB.WithContext(ctx).Where("tombstoned = ? AND cleanup_pending = ?", true, true).
		Order("updated_at ASC").Find(&rows).Error
	return rows, err
}
