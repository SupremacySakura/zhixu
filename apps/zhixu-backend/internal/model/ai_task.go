package model

import "time"

// AiTaskStatus 是 AI 工作流任务的统一状态机：pending → running → succeeded/failed。
// 所有 AI 工作流（整理、关联、演化）共用这一套状态，避免每个工作流各自造状态。
type AiTaskStatus string

const (
	AiTaskPending   AiTaskStatus = "pending"
	AiTaskRunning   AiTaskStatus = "running"
	AiTaskSucceeded AiTaskStatus = "succeeded"
	AiTaskFailed    AiTaskStatus = "failed"
)

// AI 任务类型。P3 先落地 organize，P4 的 relation / evolve 复用同一张表登记。
const (
	AiTaskTypeOrganize = "organize"
	AiTaskTypeRelation = "relation"
	AiTaskTypeEvolve   = "evolve"
)

// AiTask 是 AI 工作流的统一任务载体。
//
// DedupeKey 只在活跃期（pending/running）非空，进入终态时置 NULL：MySQL 唯一索引容许多个
// NULL，因此同一目标可以保留多条历史任务，而同一时刻只允许一个活跃任务，重复入队天然幂等。
// Payload 承载工作流产物：整理草稿在用户采纳前只存在这里，不污染知识正式字段。
type AiTask struct {
	ID       uint         `gorm:"primaryKey" json:"id"`
	UserID   uint         `gorm:"not null;index:idx_ai_task_user,priority:1" json:"-"`
	TaskType string       `gorm:"type:varchar(32);not null;index:idx_ai_task_user,priority:2" json:"task_type"`
	TargetID uint         `gorm:"not null;index:idx_ai_task_target" json:"target_id"`
	Status   AiTaskStatus `gorm:"type:varchar(16);not null;index:idx_ai_task_user,priority:3" json:"status"`
	// Payload 是该工作流的产物 JSON（整理草稿、关联候选等），默认空对象。
	Payload      string `gorm:"type:json;not null" json:"payload"`
	ErrorMessage string `gorm:"type:varchar(512);not null;default:''" json:"error_message"`
	RetryCount   int    `gorm:"not null;default:0" json:"retry_count"`
	// DedupeKey 是活跃任务的幂等键（<user>:<type>:<target>），终态置 NULL 释放。
	DedupeKey      *string    `gorm:"type:varchar(96);uniqueIndex:uniq_ai_task_active" json:"-"`
	LeaseToken     string     `gorm:"type:char(32);not null;default:''" json:"-"`
	LeaseExpiresAt *time.Time `gorm:"index:idx_ai_task_lease" json:"-"`
	NextRunAt      time.Time  `gorm:"not null;index:idx_ai_task_run" json:"-"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (AiTask) TableName() string { return "ai_tasks" }

// Active 报告任务是否仍占用幂等键。
func (t AiTask) Active() bool {
	return t.Status == AiTaskPending || t.Status == AiTaskRunning
}
