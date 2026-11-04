package model

import (
	"errors"
	"time"
)

var ErrConversationDeleted = errors.New("conversation deleted")

// ChatConversationState is both the persistent deletion tombstone and the row lock
// shared by provenance saves and deletion. Completed tombstones are never removed.
// The composite primary key serializes even the first concurrent save/delete.
type ChatConversationState struct {
	UserID         uint   `gorm:"primaryKey;autoIncrement:false"`
	ConversationID string `gorm:"primaryKey;size:64"`
	Tombstoned     bool   `gorm:"not null;default:false"`
	CleanupPending bool   `gorm:"not null;default:false;index"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ChatTurnSource records retrieval provenance for an exact Mastra assistant message.
// Legacy rows without MessageID are deliberately not associated by position.
type ChatTurnSource struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	ConversationID string    `gorm:"size:64;not null;index:idx_chat_turn_conv" json:"conversation_id"`
	UserID         uint      `gorm:"not null;index" json:"-"`
	MessageID      string    `gorm:"size:191" json:"message_id"`
	Query          string    `gorm:"size:512;not null" json:"query"`
	Sources        string    `gorm:"type:json;not null" json:"sources"` // JSON array of KnowledgeSource
	CreatedAt      time.Time `json:"created_at"`
}

// KnowledgeSource is one retrieved chunk handed to the answering model and shown to the
// user as a citation. Mirrored from the AI service's rag contract.
type KnowledgeSource struct {
	KnowledgeID uint    `json:"knowledge_id"`
	Title       string  `json:"title"`
	Section     string  `json:"section"`
	Content     string  `json:"content"`
	Score       float64 `json:"score"`
}

// ChatTurnUsage records the provider-reported token usage of one completed turn. It is
// the accounting foundation for future quotas and billing; the AI gateway only reports,
// Go owns the ledger so usage survives AI-service restarts and is scoped per user.
type ChatTurnUsage struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"not null;index" json:"-"`
	ConversationID string    `gorm:"size:64;not null;index:idx_chat_usage_conv" json:"conversation_id"`
	MessageID      string    `gorm:"size:191" json:"message_id"`
	ModelID        uint      `gorm:"not null" json:"model_id"`
	Model          string    `gorm:"size:200;not null" json:"model"`
	InputTokens    int       `gorm:"not null;default:0" json:"input_tokens"`
	OutputTokens   int       `gorm:"not null;default:0" json:"output_tokens"`
	TotalTokens    int       `gorm:"not null;default:0" json:"total_tokens"`
	CreatedAt      time.Time `json:"created_at"`
}
