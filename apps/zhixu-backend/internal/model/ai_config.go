package model

import "time"

type ModelProvider struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"not null;uniqueIndex:uniq_provider_user_key" json:"-"`
	ProviderID string    `gorm:"size:64;not null;uniqueIndex:uniq_provider_user_key" json:"provider_id"`
	Name       string    `gorm:"size:60;not null" json:"name"`
	CatalogID  string    `gorm:"size:32;not null" json:"catalog_id"`
	BaseURL    string    `gorm:"size:2048;not null" json:"base_url"`
	Protocol   string    `gorm:"size:32;not null" json:"protocol"`
	AuthType   string    `gorm:"size:16;not null" json:"auth_type"`
	Ciphertext string    `gorm:"type:text" json:"-"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ProviderModel struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"not null;index" json:"-"`
	ProviderID uint      `gorm:"not null;uniqueIndex:uniq_provider_model_name" json:"provider_id"`
	ModelID    string    `gorm:"size:200;not null;uniqueIndex:uniq_provider_model_name" json:"model_id"`
	Kind       string    `gorm:"size:16;not null" json:"kind"`
	Dimension  uint      `gorm:"not null" json:"dimension"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type UserAISettings struct {
	UserID           uint      `gorm:"primaryKey;autoIncrement:false" json:"-"`
	ChatModelID      uint      `gorm:"not null" json:"chat_model_id"`
	EmbeddingModelID uint      `gorm:"not null" json:"embedding_model_id"`
	RagTopK          int       `gorm:"not null;default:5" json:"rag_top_k"`
	RagMinScore      float64   `gorm:"not null;default:0.5" json:"rag_min_score"`
	ConfigVersion    uint64    `gorm:"not null" json:"config_version"`
	EmbeddingVersion uint64    `gorm:"not null" json:"embedding_version"`
	NeedsReindex     bool      `gorm:"not null" json:"needs_reindex"`
	UpdatedAt        time.Time `json:"updated_at"`
}
