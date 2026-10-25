package model

import "time"

type VectorState string

const (
	VectorPending VectorState = "pending"
	VectorRunning VectorState = "running"
	VectorReady   VectorState = "ready"
	VectorPartial VectorState = "partial"
)

type KnowledgeChunk struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	KnowledgeID      uint      `gorm:"not null;uniqueIndex:uniq_knowledge_chunk" json:"-"`
	UserID           uint      `gorm:"not null;index" json:"-"`
	Seq              int       `gorm:"not null;uniqueIndex:uniq_knowledge_chunk" json:"seq"`
	Section          string    `gorm:"type:text;not null" json:"section"`
	SectionLevel     int       `gorm:"not null" json:"section_level"`
	Content          string    `gorm:"type:text;not null" json:"content"`
	Chars            int       `gorm:"not null" json:"chars"`
	StartChar        int       `gorm:"not null" json:"start_char"`
	EndChar          int       `gorm:"not null" json:"end_char"`
	Overlap          bool      `gorm:"not null" json:"overlap"`
	OverlapChars     int       `gorm:"not null" json:"overlap_chars"`
	State            string    `gorm:"size:16;not null" json:"state"`
	ErrorMessage     string    `gorm:"type:text;not null" json:"error_message"`
	EmbeddingVersion uint64    `gorm:"not null" json:"embedding_version"`
	CreatedAt        time.Time `json:"-"`
	UpdatedAt        time.Time `json:"-"`
}

// VectorCleanupTask is a durable tombstone, committed before knowledge rows disappear.
// Knowledge IDs are never reused; retries only call the unmetered vector database.
type VectorCleanupTask struct {
	ID            uint      `gorm:"primaryKey"`
	UserID        uint      `gorm:"not null;uniqueIndex:uniq_vector_cleanup"`
	KnowledgeID   uint      `gorm:"not null;uniqueIndex:uniq_vector_cleanup"`
	NextAttemptAt time.Time `gorm:"not null;index"`
	CreatedAt     time.Time
}

type VectorizeTask struct {
	ID               uint        `gorm:"primaryKey"`
	KnowledgeID      uint        `gorm:"not null;uniqueIndex"`
	UserID           uint        `gorm:"not null;index"`
	Status           VectorState `gorm:"size:16;not null;index:idx_vector_queue"`
	Attempts         uint        `gorm:"not null"`
	NextAttemptAt    *time.Time  `gorm:"index:idx_vector_queue"`
	LeaseToken       string      `gorm:"size:32;not null"`
	LeaseExpiresAt   *time.Time  `gorm:"index:idx_vector_queue"`
	ModelID          uint        `gorm:"not null"`
	ConfigVersion    uint64      `gorm:"not null"`
	EmbeddingVersion uint64      `gorm:"not null"`
	Collection       string      `gorm:"size:128;not null"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
