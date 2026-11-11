package model

import "time"

// RetrievalLog records one retrieval call: the experiment data source for Recall@K,
// MRR and hit/miss distribution. Rows are append-only and scoped to the caller.
type RetrievalLog struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"not null;index:idx_retrieval_user_time,priority:1" json:"user_id"`
	ConversationID string    `gorm:"size:64;not null;default:'';index" json:"conversation_id"`
	Query          string    `gorm:"size:512;not null" json:"query"`
	Mode           string    `gorm:"size:16;not null" json:"mode"`
	Hits           string    `gorm:"type:json;not null" json:"hits"` // JSON array of {chunk_id, score, source}
	HitCount       int       `gorm:"not null" json:"hit_count"`
	LatencyMS      int       `gorm:"not null" json:"latency_ms"`
	CreatedAt      time.Time `gorm:"index:idx_retrieval_user_time,priority:2" json:"created_at"`
}

func (RetrievalLog) TableName() string { return "retrieval_logs" }
