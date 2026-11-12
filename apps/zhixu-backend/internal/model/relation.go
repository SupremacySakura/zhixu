package model

import "time"

// RelationType 知识关系类型。related、contrasts 与 contradicts 对称，其余三种严格有向。
type RelationType string

const (
	RelationTypeRelated     RelationType = "related"
	RelationTypeDependsOn   RelationType = "depends_on"
	RelationTypeSupplements RelationType = "supplements"
	RelationTypeContrasts   RelationType = "contrasts"
	RelationTypeUpdates     RelationType = "updates"
	// RelationTypeContradicts 表示两条知识就同一事实给出互斥说法（ADR-0002 推翻 ADR-0001 的合并决定）。
	// 只能由用户手动建立或从冲突提醒建立，AI 关联推荐不得产出该类型。
	RelationTypeContradicts RelationType = "contradicts"
)

// relationTypeSpec 是一个关系类型的全部语义标记。
//
// 三种查询（是否合法、能否由 AI 推荐、是否对称）共用同一张表：分开写三段级联时，
// 新增一种类型容易只改到其中一处，静默地让白名单与校验口径漂移。
// 不在表里的类型取零值，三项皆为 false。
type relationTypeSpec struct {
	// valid 表示是支持的类型；接口入参校验用。
	valid bool
	// recommendable 表示允许由 AI 关联推荐产出。它必须独立于 valid：
	// 矛盾是「两说互斥」的判断，只有人（或冲突提醒的裁决）能下定论，
	// 模型凭向量相似度猜出来的矛盾会把正常的知识关联污染成互相打架的一对。
	recommendable bool
	// symmetric 表示入库前按两侧知识 ID 的 min/max 归一，保证同一对知识只有一行，
	// 无论从哪一侧发起。
	symmetric bool
}

var relationTypeSpecs = map[RelationType]relationTypeSpec{
	RelationTypeRelated:     {valid: true, recommendable: true, symmetric: true},
	RelationTypeDependsOn:   {valid: true, recommendable: true},
	RelationTypeSupplements: {valid: true, recommendable: true},
	RelationTypeContrasts:   {valid: true, recommendable: true, symmetric: true},
	RelationTypeUpdates:     {valid: true, recommendable: true},
	// contradicts 只允许人（或冲突提醒的裁决）建立，因此不可推荐。
	RelationTypeContradicts: {valid: true, symmetric: true},
}

// spec 查表取该类型的语义标记；未登记的类型返回零值。
func (t RelationType) spec() relationTypeSpec { return relationTypeSpecs[t] }

// Valid 判断是否为支持的关系类型；接口入参校验用。
func (t RelationType) Valid() bool { return t.spec().valid }

// Recommendable 报告该类型是否允许由 AI 关联推荐产出。
func (t RelationType) Recommendable() bool { return t.spec().recommendable }

// Symmetric 报告该类型是否对称。
func (t RelationType) Symmetric() bool { return t.spec().symmetric }

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
