package model

import "time"

// RelationType 知识关系类型。related 与 contrasts 对称，其余三种严格有向。
type RelationType string

const (
	RelationTypeRelated     RelationType = "related"
	RelationTypeDependsOn   RelationType = "depends_on"
	RelationTypeSupplements RelationType = "supplements"
	RelationTypeContrasts   RelationType = "contrasts"
	RelationTypeUpdates     RelationType = "updates"
)

// Valid 判断是否为支持的关系类型；接口入参校验用。
func (t RelationType) Valid() bool {
	switch t {
	case RelationTypeRelated, RelationTypeDependsOn, RelationTypeSupplements, RelationTypeContrasts, RelationTypeUpdates:
		return true
	default:
		return false
	}
}

// Symmetric 报告该类型是否对称：对称关系入库前按两侧知识 ID 的 min/max 归一，
// 保证同一对知识只有一行，无论从哪一侧发起。
func (t RelationType) Symmetric() bool {
	return t == RelationTypeRelated || t == RelationTypeContrasts
}

// RelationStatus 关系的确认状态。suggested 是 AI 推荐待确认，rejected 保留行用于抑制重复推荐。
type RelationStatus string

const (
	RelationSuggested RelationStatus = "suggested"
	RelationConfirmed RelationStatus = "confirmed"
	RelationRejected  RelationStatus = "rejected"
)

// RelationSource 关系来源：AI 推荐或用户手动建立。
type RelationSource string

const (
	RelationSourceAI     RelationSource = "ai"
	RelationSourceManual RelationSource = "manual"
)

// KnowledgeRelation 两条知识之间的一条有向关系。
// 唯一键（user_id, source_id, target_id, relation_type）保证同一类型在同一对知识上只有一行；
// 同一对知识可以同时存在不同类型的已确认关系。
type KnowledgeRelation struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	UserID       uint           `gorm:"not null;uniqueIndex:uniq_relation_pair;index:idx_relation_user_created" json:"-"`
	SourceID     uint           `gorm:"not null;uniqueIndex:uniq_relation_pair;index:idx_relation_source" json:"source_id"`
	TargetID     uint           `gorm:"not null;uniqueIndex:uniq_relation_pair;index:idx_relation_target" json:"target_id"`
	RelationType RelationType   `gorm:"type:varchar(16);not null;uniqueIndex:uniq_relation_pair" json:"relation_type"`
	Status       RelationStatus `gorm:"type:varchar(16);not null;default:'suggested';index" json:"status"`
	Source       RelationSource `gorm:"type:varchar(16);not null;default:'manual'" json:"source"`
	Confidence   float64        `gorm:"not null;default:0" json:"confidence"`
	Evidence     string         `gorm:"type:text;not null" json:"evidence"`
	ConfirmedAt  *time.Time     `json:"confirmed_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

func (KnowledgeRelation) TableName() string { return "knowledge_relations" }
