package model

import "time"

type KnowledgeStatus string

const (
	KnowledgePending KnowledgeStatus = "pending"
	KnowledgeRunning KnowledgeStatus = "running"
	KnowledgeReady   KnowledgeStatus = "ready"
	KnowledgeFailed  KnowledgeStatus = "failed"
)

// KnowledgeSourceTypeManual 标记手动创建的知识：无来源文件，正文即用户输入。
// 与 extractor.SourceType() 的文件格式值（markdown/pdf/...）共存于 source_type。
const KnowledgeSourceTypeManual = "manual"

// KnowledgeSourceTypeWeb 标记从网页导入的知识：来源地址记录在 source_url。
const KnowledgeSourceTypeWeb = "web"

// KnowledgeItem preserves a source snapshot even if the original file is deleted.
type KnowledgeItem struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	UserID uint `gorm:"not null;uniqueIndex:uniq_knowledge_source;index:idx_knowledge_user_created" json:"-"`
	// 手动创建无来源文件，SourceFileID 为 NULL；MySQL 唯一索引容许多个 NULL，不影响文件来源幂等。
	SourceFileID     *uint  `gorm:"uniqueIndex:uniq_knowledge_source" json:"source_file_id"`
	SourceFileName   string `gorm:"type:varchar(255);not null" json:"source_file_name"`
	SourceFolderID   uint   `json:"source_folder_id"`
	SourceFolderName string `gorm:"type:text" json:"source_folder_name"`
	SourceHash       string `gorm:"type:varchar(128);not null" json:"-"`
	SourceType       string `gorm:"type:varchar(32);not null" json:"source_type"`
	// SourceURL 网页导入的来源地址；文件与手动创建为空。
	SourceURL string `gorm:"type:varchar(2048);not null;default:''" json:"source_url"`
	Title     string `gorm:"type:varchar(200);not null" json:"title"`
	Content   string `gorm:"type:longtext;not null" json:"content"`
	Version   uint   `gorm:"not null;default:1" json:"version"`
	// Summary 与 Keywords 由 AI 整理草稿被采纳后写入；未采纳前保持空串，列表与详情都不会展示 AI 未确认内容。
	// 刻意用 varchar 而不是 JSON 列：给已有表追加 NOT NULL 的 JSON 列时 MySQL 会用空串回填历史行并报 ERROR 3140。
	Summary  string `gorm:"type:varchar(1000);not null;default:''" json:"summary"`
	Keywords string `gorm:"type:varchar(1000);not null;default:''" json:"-"`
	// KeywordList 是 Keywords 的解析视图，由服务层填充，不落库。
	KeywordList []string `gorm:"-" json:"keywords"`
	// IsFavorite 收藏标记；ArchivedAt 非空表示已归档，归档知识不参与检索与默认列表。
	IsFavorite       bool            `gorm:"not null;default:false" json:"is_favorite"`
	ArchivedAt       *time.Time      `gorm:"index" json:"archived_at"`
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
