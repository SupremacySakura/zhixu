package model

import "time"

// KnowledgeChangeSource 标记版本快照的产生来源：用户手动编辑或 AI 整理回写。
type KnowledgeChangeSource string

const (
	KnowledgeChangeSourceManual KnowledgeChangeSource = "manual"
	KnowledgeChangeSourceAI     KnowledgeChangeSource = "ai"
)

// KnowledgeVersion 保存知识正文的历史快照。
// 每次编辑先写入编辑前的版本，再更新 knowledge_items；快照只增不改，用于回溯与恢复。
type KnowledgeVersion struct {
	ID            uint                  `gorm:"primaryKey" json:"id"`
	UserID        uint                  `gorm:"not null;index" json:"-"`
	KnowledgeID   uint                  `gorm:"not null;uniqueIndex:uniq_knowledge_version;index" json:"knowledge_id"`
	VersionNumber uint                  `gorm:"not null;uniqueIndex:uniq_knowledge_version" json:"version_number"`
	Title         string                `gorm:"type:varchar(200);not null" json:"title"`
	Content       string                `gorm:"type:longtext;not null" json:"content"`
	ChangeSource  KnowledgeChangeSource `gorm:"type:varchar(16);not null" json:"change_source"`
	CreatedAt     time.Time             `json:"created_at"`
}

func (KnowledgeVersion) TableName() string { return "knowledge_versions" }
