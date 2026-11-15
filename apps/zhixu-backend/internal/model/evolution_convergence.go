package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// 候选来源：updates 关系是用户显式声明过的强信号，向量近邻只是相似度提示。
// 放在 model 层是因为主结论的选择要用它，而主结论的选择属于收敛规则。
const (
	EvolutionOriginUpdateRelation = "update_relation"
	EvolutionOriginVectorNeighbor = "vector_neighbor"
)

// 演化提醒的收敛规则放在 model 层，而不是 service 层。
//
// 原因：收敛要同时被 service（检测产出提醒时）和 database（历史回填时）调用。
// 规则若留在 service，database 就必须导入 service，形成 database → service 的分层倒置
// ——那正是 router → handler → service → repository → database 这条链路要避免的。
// 这些函数只依赖提醒结构本身，不碰数据库、不调外部服务，因此属于 model 的职责。

// EvolutionConclusion 是一条提醒承载的单个结论。
//
// TriggerID 记录这个结论是「以谁为本次检查的知识」得出的。同一对知识从两个方向检查会得到
// 两条 TriggerID 相反的结论，用户因此能看到双方各自的判定，而不是只看一侧。
type EvolutionConclusion struct {
	Type            EvolutionAlertType     `json:"type"`
	TriggerID       uint                   `json:"trigger_id"`
	Reason          string                 `json:"reason"`
	Evidence        []EvolutionEvidence    `json:"evidence"`
	SuggestedAction EvolutionAction        `json:"suggested_action"`
	Severity        EvolutionAlertSeverity `json:"severity"`
	RevisedTitle    string                 `json:"revised_title,omitempty"`
	RevisedContent  string                 `json:"revised_content,omitempty"`
	// Origin 记录这条结论来自哪类候选：更新关系是用户显式声明的「谁更新谁」，
	// 向量近邻只是相似。主结论的选择要用它——用户声明的过时不该被模型的重复猜测顶掉。
	Origin string `json:"origin,omitempty"`
}

// EvolutionEvidence 是一条证据：某条知识正文里的原话。
// 引文必须能在正文中定位，定位不到就不该作为证据展示给用户。
type EvolutionEvidence struct {
	KnowledgeID uint   `json:"knowledge_id"`
	Quote       string `json:"quote"`
}

// EvolutionPayload 是提醒的结构化判定结果，整体存在提醒行的 payload 列里。
// 冲突场景额外带修订稿；修订稿缺失时前端把「采纳修订」置灰。
type EvolutionPayload struct {
	Type            EvolutionAlertType  `json:"type"`
	Reason          string              `json:"reason"`
	Evidence        []EvolutionEvidence `json:"evidence"`
	SuggestedAction EvolutionAction     `json:"suggested_action"`
	RevisedTitle    string              `json:"revised_title,omitempty"`
	RevisedContent  string              `json:"revised_content,omitempty"`
	// Conclusions 是这条提醒承载的全部结论。顶层字段始终跟随主结论，
	// 因此只读顶层字段的旧前端与旧评测仍能拿到一条完整结论；结论集合供详情页展示另一侧判定。
	Conclusions []EvolutionConclusion `json:"conclusions,omitempty"`
}

// EvolutionPairKey 返回知识对的身份：两个知识 ID 排序后取哈希。
// 顺序无关是这里唯一的职责——A 检查 B 与 B 检查 A 必须落在同一行上。
func EvolutionPairKey(first, second uint) string {
	low, high := first, second
	if low > high {
		low, high = high, low
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("pair|%d|%d", low, high)))
	return hex.EncodeToString(sum[:])
}

// ParseEvolutionPayload 解析载荷；脏数据返回空载荷，不让列表整体失败。
func ParseEvolutionPayload(raw string) EvolutionPayload {
	var payload EvolutionPayload
	if raw == "" {
		return payload
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return EvolutionPayload{}
	}
	return payload
}

// EncodeEvolutionPayload 把载荷序列化。
//
// 失败时返回错误而不是空对象：吞掉错误写一条载荷为 "{}" 的提醒，用户点开只会看到
// 一条没有理由、没有证据的空提醒，比不写更糟。调用方据此丢弃整条提醒。
func EncodeEvolutionPayload(payload EvolutionPayload) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// MarshalEvolutionPayload 让载荷顶层字段跟随主结论，旧前端与旧评测读顶层字段时仍能拿到结论。
// 返回错误由调用方决定如何处理：仓储侧丢弃整条提醒，任务侧放弃本次落库。
func MarshalEvolutionPayload(primary EvolutionConclusion, conclusions []EvolutionConclusion) (string, error) {
	return EncodeEvolutionPayload(EvolutionPayload{
		Type:            primary.Type,
		Reason:          primary.Reason,
		Evidence:        primary.Evidence,
		SuggestedAction: primary.SuggestedAction,
		RevisedTitle:    primary.RevisedTitle,
		RevisedContent:  primary.RevisedContent,
		Conclusions:     conclusions,
	})
}

// ConclusionsOf 取出载荷里的结论集合。历史载荷只有顶层字段、没有结论集合时，
// 用顶层字段合成一条，保证旧数据合并时不会丢掉唯一的那条结论。
func ConclusionsOf(payload EvolutionPayload) []EvolutionConclusion {
	if len(payload.Conclusions) > 0 {
		out := make([]EvolutionConclusion, len(payload.Conclusions))
		copy(out, payload.Conclusions)
		// 顶层修订稿比结论集合更新时（例如旧版本只写了顶层字段），补回主结论，
		// 否则合并会把这条修订稿连同「采纳修订」一起弄丢。
		for i := range out {
			if out[i].Type != payload.Type {
				continue
			}
			if payload.RevisedContent != "" && out[i].RevisedContent == "" {
				out[i].RevisedTitle = payload.RevisedTitle
				out[i].RevisedContent = payload.RevisedContent
			}
			return out
		}
		return out
	}
	if !payload.Type.Valid() {
		return nil
	}
	return []EvolutionConclusion{{
		Type:            payload.Type,
		Reason:          payload.Reason,
		Evidence:        payload.Evidence,
		SuggestedAction: payload.SuggestedAction,
		Severity:        EvolutionSeverityReview,
		RevisedTitle:    payload.RevisedTitle,
		RevisedContent:  payload.RevisedContent,
	}}
}

// ConclusionRank 给结论排序：更新关系给出的过时结论排在最前，其余按重复 → 冲突 → 过时。
//
// 为什么把「更新关系的过时」提到最高：那是用户自己声明过的新旧关系，属于已确认的事实；
// 同一对知识上模型再猜出重复，处置会是「归档其一」，可能把较新的那条归档掉。
// 让用户声明的事实定调，模型的不同意见降级为次要结论展示，才既保留信息又不误导操作。
func ConclusionRank(conclusion EvolutionConclusion) int {
	if conclusion.Origin == EvolutionOriginUpdateRelation && conclusion.Type == EvolutionAlertTypeOutdated {
		return -1
	}
	return conclusion.Type.Priority()
}

// PrimaryConclusion 选出主结论；同级时保留先出现的一条。
// 排序稳定，保证同一组结论每次算出的主结论一致。
func PrimaryConclusion(conclusions []EvolutionConclusion) EvolutionConclusion {
	best := conclusions[0]
	for _, candidate := range conclusions[1:] {
		if ConclusionRank(candidate) < ConclusionRank(best) {
			best = candidate
		}
	}
	return best
}

// MergeEvolutionConclusions 合并两侧结论并按去重键收敛，主结论排在第一位。
//
// 排序口径必须与 PrimaryConclusion 一致（都用 ConclusionRank），否则
// conclusions[0] 会与 alert_type 指向不同的结论：前端按 index === 0 标注
// 「当前结论」，评测也按顺序取数，两者都会读到错的那条。
func MergeEvolutionConclusions(sets ...[]EvolutionConclusion) []EvolutionConclusion {
	seen := make(map[string]bool)
	out := make([]EvolutionConclusion, 0)
	for _, set := range sets {
		for _, conclusion := range set {
			if !conclusion.Type.Valid() {
				continue
			}
			key := conclusion.mergeKey()
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, conclusion)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return ConclusionRank(out[i]) < ConclusionRank(out[j])
	})
	return out
}

// mergeKey 是结论的去重键：同一方向、同一类型、同一组证据视为同一条结论。
// 不含理由：模型换个措辞重述同一判断时不该堆出第二条。
func (c EvolutionConclusion) mergeKey() string {
	return fmt.Sprintf("%s|%d|%s", c.Type, c.TriggerID, EvolutionEvidenceFingerprint(c.Evidence))
}

// EvolutionEvidenceFingerprint 把证据数组压成一个稳定字符串：
// 先按知识 ID、再按引文排序，避免同一组证据因顺序不同算出两个指纹。
func EvolutionEvidenceFingerprint(evidence []EvolutionEvidence) string {
	items := make([]EvolutionEvidence, len(evidence))
	copy(items, evidence)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].KnowledgeID != items[j].KnowledgeID {
			return items[i].KnowledgeID < items[j].KnowledgeID
		}
		return items[i].Quote < items[j].Quote
	})
	var builder strings.Builder
	for _, item := range items {
		builder.WriteString(fmt.Sprintf("%d:%s\n", item.KnowledgeID, item.Quote))
	}
	return builder.String()
}

// EvolutionDedupeKey 计算提醒指纹：类型 + 排序后的涉及知识 ID + 证据引文指纹。
// 与 AI 任务的幂等键不同，这个指纹在终态也保留，用来抑制同一证据重复弹提醒；
// 引文随正文变化时指纹随之改变，于是同一条知识的新问题会被当成新提醒。
func EvolutionDedupeKey(alertType EvolutionAlertType, subjectIDs []uint, evidence []EvolutionEvidence) string {
	ids := make([]uint, len(subjectIDs))
	copy(ids, subjectIDs)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%d", id))
	}
	raw := fmt.Sprintf("%s|%s|%s", alertType, strings.Join(parts, ","), EvolutionEvidenceFingerprint(evidence))
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// MergeEvolutionAlert 把一条新提醒并入同一知识对已有的提醒。
//
// 这是提醒收敛的唯一规则来源，不碰数据库：仓储层负责「按知识对读出来 → 调它 → 写回去」，
// 唯一索引只作并发安全网。existing 为 nil 表示这一对还没有提醒。
//
// 返回 nil 提醒表示本次不写库（已裁决且正文未变）。
func MergeEvolutionAlert(existing, incoming *EvolutionAlert) (*EvolutionAlert, EvolutionUpsertOutcome) {
	if existing == nil {
		return incoming, EvolutionAlertCreated
	}
	versionsChanged := pairVersionsDiffer(existing.PairVersionMap(), incoming.PairVersionMap())

	merged := *existing
	merged.PairVersions = incoming.PairVersions
	merged.DedupeKey = incoming.DedupeKey
	// 方向跟着最近一次检查走：SubjectIDs[0] 始终是本次触发的知识，前端左右并排才对得上。
	merged.SubjectIDs = incoming.SubjectIDs

	conclusions := MergeEvolutionConclusions(
		ConclusionsOf(ParseEvolutionPayload(existing.Payload)),
		ConclusionsOf(ParseEvolutionPayload(incoming.Payload)),
	)
	primary := PrimaryConclusion(conclusions)
	body, err := MarshalEvolutionPayload(primary, conclusions)
	if err != nil {
		// 序列化失败说明载荷本身有问题，写一条空提醒比不写更糟：丢弃本次合并。
		return nil, EvolutionUpsertOutcome("")
	}
	merged.Payload = body
	merged.AlertType = primary.Type
	merged.Severity = MergeEvolutionSeverity(conclusions)

	switch {
	case existing.Status.Open():
		// 待处理与暂不处理都留在待办视野：暂不处理回到待处理，待处理只刷新。
		if existing.Status == EvolutionAlertDeferred {
			merged.Status = EvolutionAlertPending
			merged.ResolvedAt = nil
			merged.ResolvedAction = ""
			merged.ResolutionNote = ""
			return &merged, EvolutionAlertRevived
		}
		merged.Status = EvolutionAlertPending
		return &merged, EvolutionAlertRefreshed
	case !versionsChanged:
		// 已裁决且正文没变：用户的裁决针对的就是这两份正文，本次抑制。
		return nil, EvolutionAlertSuppressed
	default:
		// 正文变了，裁决的前提不成立：回到待处理，并把上一次的裁决留作历史。
		merged.Status = EvolutionAlertPending
		merged.ResolvedAt = nil
		merged.ResolvedAction = ""
		merged.ResolutionNote = mergeResolutionHistory(existing)
		return &merged, EvolutionAlertRevived
	}
}

// MergeEvolutionSeverity 只要有一条结论证据充分，整条提醒就按强展示：用户点开能看到强证据。
func MergeEvolutionSeverity(conclusions []EvolutionConclusion) EvolutionAlertSeverity {
	for _, conclusion := range conclusions {
		if conclusion.Severity == EvolutionSeverityStrong {
			return EvolutionSeverityStrong
		}
	}
	return EvolutionSeverityReview
}

// pairVersionsDiffer 报告两侧正文版本是否有变化；任一侧缺失按「变了」处理，
// 宁可多提醒一次，也不要把用户改过正文的提醒永久压住。
func pairVersionsDiffer(before, after map[uint]uint) bool {
	if len(before) == 0 || len(after) == 0 {
		return true
	}
	for id, version := range after {
		if before[id] != version {
			return true
		}
	}
	for id, version := range before {
		if after[id] != version {
			return true
		}
	}
	return false
}

// mergeResolutionHistory 把上一次裁决追加到备注里，保留可追溯的痕迹。
func mergeResolutionHistory(existing *EvolutionAlert) string {
	history := strings.TrimSpace(existing.ResolutionNote)
	action := string(existing.ResolvedAction)
	if action == "" && history == "" {
		return ""
	}
	entry := fmt.Sprintf("[%s] %s", existing.UpdatedAt.Format(time.RFC3339), action)
	if history != "" {
		entry += "：" + history
	}
	return truncateRunes(entry, 1000)
}

// truncateRunes 按字符截断，避免切碎多字节字符。
func truncateRunes(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	return string([]rune(value)[:limit])
}
