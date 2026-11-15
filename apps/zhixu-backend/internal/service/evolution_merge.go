package service

import "zhixu-backend/internal/model"

// 收敛规则的实现在 model 层，这里只做类型别名。
//
// 别名而不是包装函数：包一层转发会让每个调用点多一跳，也让「同一规则只有一个来源」
// 变得不明显。别名让 service 与 database 共用同一份定义，同时保持调用方写法不变。
type (
	// EvolutionConclusion 是一条提醒承载的单个结论。
	EvolutionConclusion = model.EvolutionConclusion
	// EvolutionEvidence 是一条证据：某条知识正文里的原话。
	EvolutionEvidence = model.EvolutionEvidence
	// EvolutionPayload 是提醒的结构化判定结果。
	EvolutionPayload = model.EvolutionPayload
)

// EvolutionPairKey 返回知识对的身份：两个知识 ID 排序后取哈希。
func EvolutionPairKey(first, second uint) string { return model.EvolutionPairKey(first, second) }

// MergeEvolutionAlert 把一条新提醒并入同一知识对已有的提醒。
func MergeEvolutionAlert(existing, incoming *model.EvolutionAlert) (*model.EvolutionAlert, model.EvolutionUpsertOutcome) {
	return model.MergeEvolutionAlert(existing, incoming)
}

// ParseEvolutionPayload 解析载荷；脏数据返回空载荷，不让列表整体失败。
func ParseEvolutionPayload(raw string) EvolutionPayload { return model.ParseEvolutionPayload(raw) }

// EncodeEvolutionPayload 把载荷序列化，失败返回错误而不是空对象。
func EncodeEvolutionPayload(payload EvolutionPayload) (string, error) {
	return model.EncodeEvolutionPayload(payload)
}

// MarshalEvolutionPayload 让载荷顶层字段跟随主结论。
func MarshalEvolutionPayload(primary EvolutionConclusion, conclusions []EvolutionConclusion) (string, error) {
	return model.MarshalEvolutionPayload(primary, conclusions)
}

// ConclusionsOf 取出载荷里的结论集合，历史载荷按顶层字段合成一条。
func ConclusionsOf(payload EvolutionPayload) []EvolutionConclusion {
	return model.ConclusionsOf(payload)
}

// ConclusionRank 给结论排序：更新关系的过时最高，其余按重复 → 冲突 → 过时。
func ConclusionRank(conclusion EvolutionConclusion) int { return model.ConclusionRank(conclusion) }

// PrimaryConclusion 选出主结论；同级时保留先出现的一条。
func PrimaryConclusion(conclusions []EvolutionConclusion) EvolutionConclusion {
	return model.PrimaryConclusion(conclusions)
}

// MergeEvolutionConclusions 合并两侧结论并按去重键收敛，主结论排在第一位。
func MergeEvolutionConclusions(sets ...[]EvolutionConclusion) []EvolutionConclusion {
	return model.MergeEvolutionConclusions(sets...)
}

// EvolutionDedupeKey 计算提醒的证据指纹。
func EvolutionDedupeKey(alertType model.EvolutionAlertType, subjectIDs []uint, evidence []EvolutionEvidence) string {
	return model.EvolutionDedupeKey(alertType, subjectIDs, evidence)
}

// 以下小写包装供 service 内部沿用原有写法，避免为一次分层调整改动几十处调用点。
// 它们不承载逻辑，只转发到 model——规则本身只有 model 层那一份。

func conclusionsOf(payload EvolutionPayload) []EvolutionConclusion {
	return model.ConclusionsOf(payload)
}

func conclusionRank(conclusion EvolutionConclusion) int { return model.ConclusionRank(conclusion) }

func primaryConclusion(conclusions []EvolutionConclusion) EvolutionConclusion {
	return model.PrimaryConclusion(conclusions)
}

func mergeConclusions(sets ...[]EvolutionConclusion) []EvolutionConclusion {
	return model.MergeEvolutionConclusions(sets...)
}

func mergeSeverity(conclusions []EvolutionConclusion) model.EvolutionAlertSeverity {
	return model.MergeEvolutionSeverity(conclusions)
}

func marshalEvolutionPayload(primary EvolutionConclusion, conclusions []EvolutionConclusion) (string, error) {
	return model.MarshalEvolutionPayload(primary, conclusions)
}

func encodeEvolutionPayload(payload EvolutionPayload) (string, error) {
	return model.EncodeEvolutionPayload(payload)
}
