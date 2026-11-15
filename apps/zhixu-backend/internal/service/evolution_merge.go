package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"
)

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

// EvolutionConclusion 是一条提醒承载的单个结论。
//
// TriggerID 记录这个结论是「以谁为本次检查的知识」得出的。同一对知识从两个方向检查会得到
// 两条 TriggerID 相反的结论，用户因此能看到双方各自的判定，而不是只看一侧。
type EvolutionConclusion struct {
	Type            model.EvolutionAlertType     `json:"type"`
	TriggerID       uint                         `json:"trigger_id"`
	Reason          string                       `json:"reason"`
	Evidence        []EvolutionEvidence          `json:"evidence"`
	SuggestedAction model.EvolutionAction        `json:"suggested_action"`
	Severity        model.EvolutionAlertSeverity `json:"severity"`
	RevisedTitle    string                       `json:"revised_title,omitempty"`
	RevisedContent  string                       `json:"revised_content,omitempty"`
}

// mergeKey 是结论的去重键：同一方向、同一类型、同一组证据视为同一条结论。
// 不含理由：模型换个措辞重述同一判断时不该堆出第二条。
func (c EvolutionConclusion) mergeKey() string {
	return fmt.Sprintf("%s|%d|%s", c.Type, c.TriggerID, evolutionEvidenceFingerprint(c.Evidence))
}

// conclusionsOf 取出载荷里的结论集合。历史载荷只有顶层字段、没有结论集合时，
// 用顶层字段合成一条，保证旧数据合并时不会丢掉唯一的那条结论。
func conclusionsOf(payload EvolutionPayload) []EvolutionConclusion {
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
		Severity:        model.EvolutionSeverityReview,
		RevisedTitle:    payload.RevisedTitle,
		RevisedContent:  payload.RevisedContent,
	}}
}

// primaryConclusion 按重复 → 冲突 → 过时 选出主结论；同级时保留先出现的一条。
// 排序稳定，保证同一组结论每次算出的主结论一致。
func primaryConclusion(conclusions []EvolutionConclusion) EvolutionConclusion {
	best := conclusions[0]
	for _, candidate := range conclusions[1:] {
		if candidate.Type.Priority() < best.Type.Priority() {
			best = candidate
		}
	}
	return best
}

// MergeEvolutionAlert 把一条新提醒并入同一知识对已有的提醒。
//
// 这是提醒收敛的唯一规则来源，不碰数据库：仓储层负责「按知识对读出来 → 调它 → 写回去」，
// 唯一索引只作并发安全网。existing 为 nil 表示这一对还没有提醒。
//
// 返回 nil 提醒表示本次不写库（已裁决且正文未变）。
func MergeEvolutionAlert(existing, incoming *model.EvolutionAlert) (*model.EvolutionAlert, repository.EvolutionUpsertOutcome) {
	if existing == nil {
		return incoming, repository.EvolutionAlertCreated
	}
	versionsChanged := pairVersionsDiffer(existing.PairVersionMap(), incoming.PairVersionMap())

	merged := *existing
	merged.PairVersions = incoming.PairVersions
	merged.DedupeKey = incoming.DedupeKey
	// 方向跟着最近一次检查走：SubjectIDs[0] 始终是本次触发的知识，前端左右并排才对得上。
	merged.SubjectIDs = incoming.SubjectIDs

	conclusions := mergeConclusions(
		conclusionsOf(ParseEvolutionPayload(existing.Payload)),
		conclusionsOf(ParseEvolutionPayload(incoming.Payload)),
	)
	primary := primaryConclusion(conclusions)
	merged.Payload = marshalEvolutionPayload(primary, conclusions)
	merged.AlertType = primary.Type
	merged.Severity = mergeSeverity(conclusions)

	switch {
	case existing.Status.Open():
		// 待处理与暂不处理都留在待办视野：暂不处理回到待处理，待处理只刷新。
		if existing.Status == model.EvolutionAlertDeferred {
			merged.Status = model.EvolutionAlertPending
			merged.ResolvedAt = nil
			merged.ResolvedAction = ""
			merged.ResolutionNote = ""
			return &merged, repository.EvolutionAlertRevived
		}
		merged.Status = model.EvolutionAlertPending
		return &merged, repository.EvolutionAlertRefreshed
	case !versionsChanged:
		// 已裁决且正文没变：用户的裁决针对的就是这两份正文，本次抑制。
		return nil, repository.EvolutionAlertSuppressed
	default:
		// 正文变了，裁决的前提不成立：回到待处理，并把上一次的裁决留作历史。
		merged.Status = model.EvolutionAlertPending
		merged.ResolvedAt = nil
		merged.ResolvedAction = ""
		merged.ResolutionNote = mergeResolutionHistory(existing)
		return &merged, repository.EvolutionAlertRevived
	}
}

// mergeConclusions 合并两侧结论并按去重键收敛，顺序保持「先出现的在前」。
func mergeConclusions(sets ...[]EvolutionConclusion) []EvolutionConclusion {
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
		return out[i].Type.Priority() < out[j].Type.Priority()
	})
	return out
}

// mergeSeverity 只要有一条结论证据充分，整条提醒就按强展示：用户点开能看到强证据。
func mergeSeverity(conclusions []EvolutionConclusion) model.EvolutionAlertSeverity {
	for _, conclusion := range conclusions {
		if conclusion.Severity == model.EvolutionSeverityStrong {
			return model.EvolutionSeverityStrong
		}
	}
	return model.EvolutionSeverityReview
}

// marshalEvolutionPayload 让载荷顶层字段跟随主结论，旧前端与旧评测读顶层字段时仍能拿到结论。
func marshalEvolutionPayload(primary EvolutionConclusion, conclusions []EvolutionConclusion) string {
	payload := EvolutionPayload{
		Type:            primary.Type,
		Reason:          primary.Reason,
		Evidence:        primary.Evidence,
		SuggestedAction: primary.SuggestedAction,
		RevisedTitle:    primary.RevisedTitle,
		RevisedContent:  primary.RevisedContent,
		Conclusions:     conclusions,
	}
	return encodeEvolutionPayload(payload)
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
func mergeResolutionHistory(existing *model.EvolutionAlert) string {
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
