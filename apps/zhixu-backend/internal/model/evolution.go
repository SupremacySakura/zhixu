package model

import (
	"encoding/json"
	"time"
)

// EvolutionAlertType 演化提醒类型：重复、过时、冲突。
// 三者共用一张表与一套裁决流程，只有可执行动作与证据形状不同。
type EvolutionAlertType string

const (
	EvolutionAlertTypeDuplicate EvolutionAlertType = "duplicate"
	EvolutionAlertTypeOutdated  EvolutionAlertType = "outdated"
	EvolutionAlertTypeConflict  EvolutionAlertType = "conflict"
)

// Valid 判断是否为支持的提醒类型。
func (t EvolutionAlertType) Valid() bool {
	switch t {
	case EvolutionAlertTypeDuplicate, EvolutionAlertTypeOutdated, EvolutionAlertTypeConflict:
		return true
	default:
		return false
	}
}

// EvolutionAlertStatus 提醒的审核状态：待处理 → 已处理/已忽略，或暂不处理（可回到待处理）。
// 与 KnowledgeStatus 无关：那是知识的处理状态，不是审核状态。
type EvolutionAlertStatus string

const (
	EvolutionAlertPending   EvolutionAlertStatus = "pending"
	EvolutionAlertResolved  EvolutionAlertStatus = "resolved"
	EvolutionAlertDismissed EvolutionAlertStatus = "dismissed"
	EvolutionAlertDeferred  EvolutionAlertStatus = "deferred"
)

// Valid 判断是否为支持的提醒状态。
func (s EvolutionAlertStatus) Valid() bool {
	switch s {
	case EvolutionAlertPending, EvolutionAlertResolved, EvolutionAlertDismissed, EvolutionAlertDeferred:
		return true
	default:
		return false
	}
}

// Open 报告该状态是否仍在待办视野内（待处理与暂不处理）。
func (s EvolutionAlertStatus) Open() bool {
	return s == EvolutionAlertPending || s == EvolutionAlertDeferred
}

// EvolutionAlertSeverity 提醒严重度：复核（默认）与强（证据充分）。
// 只做徽标展示，不参与排序。
type EvolutionAlertSeverity string

const (
	EvolutionSeverityReview EvolutionAlertSeverity = "review"
	EvolutionSeverityStrong EvolutionAlertSeverity = "strong"
)

// Valid 判断严重度取值是否合法。
func (s EvolutionAlertSeverity) Valid() bool {
	return s == EvolutionSeverityReview || s == EvolutionSeverityStrong
}

// EvolutionAction 用户对一条提醒的裁决动作。
// defer 是状态迁移而不是裁决动作：它不写裁决时间，也不终止提醒。
type EvolutionAction string

const (
	EvolutionActionAcceptRevision EvolutionAction = "accept_revision"
	EvolutionActionKeepBoth       EvolutionAction = "keep_both"
	EvolutionActionArchiveOne     EvolutionAction = "archive_one"
	EvolutionActionCreateRelation EvolutionAction = "create_relation"
	EvolutionActionMarkOutdated   EvolutionAction = "mark_outdated"
	EvolutionActionMarkVerified   EvolutionAction = "mark_verified"
	EvolutionActionDismiss        EvolutionAction = "dismiss"
	EvolutionActionDefer          EvolutionAction = "defer"
)

// Valid 判断是否为支持的裁决动作（含 defer）。
func (a EvolutionAction) Valid() bool {
	switch a {
	case EvolutionActionAcceptRevision, EvolutionActionKeepBoth, EvolutionActionArchiveOne,
		EvolutionActionCreateRelation, EvolutionActionMarkOutdated, EvolutionActionMarkVerified,
		EvolutionActionDismiss, EvolutionActionDefer:
		return true
	default:
		return false
	}
}

// AllowedFor 报告该动作对某类提醒是否有意义：语义不通的组合一律拒绝，
// 避免前端出现「对重复提醒采纳修订」这类按钮。
func (a EvolutionAction) AllowedFor(alertType EvolutionAlertType) bool {
	switch alertType {
	case EvolutionAlertTypeDuplicate:
		return a == EvolutionActionArchiveOne || a == EvolutionActionKeepBoth ||
			a == EvolutionActionMarkVerified || a == EvolutionActionDismiss || a == EvolutionActionDefer
	case EvolutionAlertTypeOutdated:
		return a == EvolutionActionMarkOutdated || a == EvolutionActionMarkVerified ||
			a == EvolutionActionDismiss || a == EvolutionActionDefer
	case EvolutionAlertTypeConflict:
		return a == EvolutionActionAcceptRevision || a == EvolutionActionKeepBoth ||
			a == EvolutionActionCreateRelation || a == EvolutionActionMarkVerified ||
			a == EvolutionActionDismiss || a == EvolutionActionDefer
	default:
		return false
	}
}

// EvolutionAlert 是一条待裁决的演化提醒。
//
// SubjectIDs 存 JSON 数组且**有序**：第 0 位是本次触发检查的知识，第 1 位是被比对的知识，
// 前端按这个顺序左右并排展示。指纹计算时改用排序后的 ID，避免同一对被判反方向时算出两个指纹。
// DedupeKey 是提醒指纹（类型 + 排序后的 ID + 证据引文），带唯一索引且**终态不清空**：
// 终态仍占位才能抑制同一证据再次弹提醒。
type EvolutionAlert struct {
	ID         uint                   `gorm:"primaryKey" json:"id"`
	UserID     uint                   `gorm:"not null;index:idx_evolution_alert_user_status,priority:1" json:"-"`
	AlertType  EvolutionAlertType     `gorm:"type:varchar(16);not null;index:idx_evolution_alert_user_status,priority:3" json:"alert_type"`
	Status     EvolutionAlertStatus   `gorm:"type:varchar(16);not null;default:'pending';index:idx_evolution_alert_user_status,priority:2" json:"status"`
	Severity   EvolutionAlertSeverity `gorm:"type:varchar(16);not null;default:'review'" json:"severity"`
	SubjectIDs string                 `gorm:"type:json;not null" json:"-"`
	// Payload 是结构化判定结果（理由、证据、建议动作、冲突时的修订稿），解析见服务层。
	Payload        string          `gorm:"type:json;not null" json:"-"`
	DedupeKey      string          `gorm:"type:char(64);not null;uniqueIndex:uniq_evolution_alert_dedupe" json:"-"`
	ResolvedAt     *time.Time      `json:"resolved_at"`
	ResolvedAction EvolutionAction `gorm:"type:varchar(32);not null;default:''" json:"resolved_action"`
	ResolutionNote string          `gorm:"type:varchar(1000);not null;default:''" json:"resolution_note"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

func (EvolutionAlert) TableName() string { return "evolution_alerts" }

// SubjectIDList 解析有序的涉及知识 ID；脏数据按空数组处理，不影响列表渲染。
func (a *EvolutionAlert) SubjectIDList() []uint {
	if a == nil || a.SubjectIDs == "" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(a.SubjectIDs), &ids); err != nil {
		return nil
	}
	return ids
}

// SetSubjectIDs 写入有序的涉及知识 ID 数组。
func (a *EvolutionAlert) SetSubjectIDs(ids []uint) error {
	encoded, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	a.SubjectIDs = string(encoded)
	return nil
}

// Active 报告提醒是否仍在待办视野内。
func (a EvolutionAlert) Active() bool { return a.Status.Open() }
