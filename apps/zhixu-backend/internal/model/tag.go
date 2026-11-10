package model

import "time"

// 标签来源：manual=用户手工维护；ai=AI 整理建议（P3 使用）。
const (
	TagSourceManual = "manual"
	TagSourceAI     = "ai"
)

// Tag 用户自定义知识标签。同一用户下名称唯一，唯一性依赖 utf8mb4
// 排序规则的大小写不敏感语义（MySQL 默认 utf8mb4_0900_ai_ci 即可）。
type Tag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;uniqueIndex:uniq_tag_user_name" json:"-"`
	Name      string    `gorm:"size:30;not null;uniqueIndex:uniq_tag_user_name" json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func (Tag) TableName() string { return "tags" }

// KnowledgeTag 知识-标签绑定；source 记录绑定来源，删除知识时级联清理绑定。
type KnowledgeTag struct {
	KnowledgeID uint      `gorm:"primaryKey" json:"knowledge_id"`
	TagID       uint      `gorm:"primaryKey" json:"tag_id"`
	UserID      uint      `gorm:"not null;index" json:"-"`
	Source      string    `gorm:"size:16;not null;default:'manual'" json:"source"`
	CreatedAt   time.Time `json:"created_at"`
}

func (KnowledgeTag) TableName() string { return "knowledge_tags" }
