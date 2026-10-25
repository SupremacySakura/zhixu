package model

import "time"

type KnowledgeStatus string

const (
	KnowledgePending KnowledgeStatus = "pending"
	KnowledgeRunning KnowledgeStatus = "running"
	KnowledgeReady   KnowledgeStatus = "ready"
	KnowledgeFailed  KnowledgeStatus = "failed"
)

// KnowledgeItem preserves a source snapshot even if the original file is deleted.
type KnowledgeItem struct {
	ID               uint            `gorm:"primaryKey" json:"id"`
	UserID           uint            `gorm:"not null;uniqueIndex:uniq_knowledge_source;index:idx_knowledge_user_created" json:"-"`
	SourceFileID     uint            `gorm:"not null;uniqueIndex:uniq_knowledge_source" json:"source_file_id"`
	SourceFileName   string          `gorm:"type:varchar(255);not null" json:"source_file_name"`
	SourceFolderID   uint            `json:"source_folder_id"`
	SourceFolderName string          `gorm:"type:text" json:"source_folder_name"`
	SourceHash       string          `gorm:"type:varchar(128);not null" json:"-"`
	SourceType       string          `gorm:"type:varchar(32);not null" json:"source_type"`
	Title            string          `gorm:"type:varchar(200);not null" json:"title"`
	Content          string          `gorm:"type:longtext;not null" json:"content"`
	Status           KnowledgeStatus `gorm:"type:varchar(16);not null;index" json:"status"`
	ErrorCode        string          `gorm:"type:varchar(64);not null" json:"error_code"`
	ErrorMessage     string          `gorm:"type:text;not null" json:"error_message"`
	VectorState      VectorState     `gorm:"size:16;not null;default:'';index" json:"vector_state"`
	ChunksTotal      int             `gorm:"not null;default:0" json:"chunks_total"`
	ChunksReady      int             `gorm:"not null;default:0" json:"chunks_ready"`
	ChunksFailed     int             `gorm:"not null;default:0" json:"chunks_failed"`
	EmbeddingModel   string          `gorm:"size:200;not null;default:''" json:"embedding_model"`
	EmbeddingVersion uint64          `gorm:"not null;default:0" json:"embedding_version"`
	VectorError      string          `gorm:"type:text" json:"vector_error"`
	ImportedAt       *time.Time      `json:"imported_at"`
	CreatedAt        time.Time       `gorm:"index:idx_knowledge_user_created" json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

func (KnowledgeItem) TableName() string { return "knowledge_items" }

// Lease fencing prevents a crashed or slow worker from overwriting a newer result.
type KnowledgeImportTask struct {
	ID             uint            `gorm:"primaryKey" json:"id"`
	KnowledgeID    uint            `gorm:"not null;uniqueIndex" json:"knowledge_id"`
	UserID         uint            `gorm:"not null;index" json:"-"`
	Status         KnowledgeStatus `gorm:"type:varchar(16);not null;index:idx_knowledge_task_queue" json:"status"`
	Attempts       uint            `gorm:"not null" json:"attempts"`
	LeaseToken     string          `gorm:"type:char(32);not null" json:"-"`
	LeaseExpiresAt *time.Time      `gorm:"index:idx_knowledge_task_queue" json:"-"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

func (KnowledgeImportTask) TableName() string { return "knowledge_import_tasks" }
