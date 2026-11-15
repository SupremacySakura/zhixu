package service

import (
	"strings"
	"testing"

	"zhixu-backend/internal/model"

	"github.com/stretchr/testify/require"
)

// 修订稿门槛：证据齐全、标题正文都在且正文不超长时才整份保留；
// 任何一项不满足就整份丢弃（半份修订稿比没有更糟），提醒随之降级为「仅提示」。
func TestSanitizeEvolveJudgementsKeepsCompleteRevisionOnly(t *testing.T) {
	candidates := []EvolveCandidate{{KnowledgeID: 21, Score: 0.82}}
	raw := func(title, content string) []rawEvolveJudgement {
		return []rawEvolveJudgement{{
			TargetID: 21, Type: string(model.EvolutionAlertTypeConflict), Reason: "两条知识对召回率给出互斥结论。",
			Evidence: []struct {
				KnowledgeID uint   `json:"knowledge_id"`
				Quote       string `json:"quote"`
			}{{KnowledgeID: 10, Quote: "同一段正文"}, {KnowledgeID: 21, Quote: "近似最近邻检索"}},
			Severity: string(model.EvolutionSeverityStrong), RevisedTitle: title, RevisedContent: content,
		}}
	}

	t.Run("keeps a complete revision", func(t *testing.T) {
		out := sanitizeEvolveJudgements(10, candidates, raw("HNSW 的召回率", "HNSW 的分层图结构在召回率上优于 IVF。"))
		require.Len(t, out, 1)
		require.Equal(t, "HNSW 的召回率", out[0].RevisedTitle)
		require.Equal(t, "HNSW 的分层图结构在召回率上优于 IVF。", out[0].RevisedContent)
	})

	t.Run("drops an oversized revision", func(t *testing.T) {
		oversized := strings.Repeat("长", EvolveRevisedContentMaxRunes+1)
		out := sanitizeEvolveJudgements(10, candidates, raw("HNSW 的召回率", oversized))
		require.Len(t, out, 1)
		require.Empty(t, out[0].RevisedTitle, "超长时整份丢弃，不截断")
		require.Empty(t, out[0].RevisedContent)
		require.Equal(t, string(model.EvolutionAlertTypeConflict), string(out[0].Type), "结论本身仍然保留")
	})

	t.Run("drops a revision without both halves", func(t *testing.T) {
		out := sanitizeEvolveJudgements(10, candidates, raw("", "HNSW 的分层图结构在召回率上优于 IVF。"))
		require.Len(t, out, 1)
		require.Empty(t, out[0].RevisedTitle)
		require.Empty(t, out[0].RevisedContent)
	})

	// 客户端清洗与处理器共用同一个门槛：标题超长先由 truncateRunes 截到 200 再判可用。
	t.Run("keeps a revision whose title only needs truncation", func(t *testing.T) {
		longTitle := strings.Repeat("标", 260)
		out := sanitizeEvolveJudgements(10, candidates, raw(longTitle, "HNSW 的分层图结构在召回率上优于 IVF。"))
		require.Len(t, out, 1)
		require.Equal(t, 200, len([]rune(out[0].RevisedTitle)), "标题按 200 字截断而不是整份丢弃")
		require.NotEmpty(t, out[0].RevisedContent)
	})
}

// 类型分派只由候选来源决定：相似度只负责筛选候选，不再参与允许类型的判断。
// 向量近邻对三类结论都不设限；更新关系是用户显式声明的强信号，只接受过时。
func TestEvolutionAllowedTypesDispatchesByOrigin(t *testing.T) {
	for _, tc := range []struct {
		name   string
		origin string
		want   []model.EvolutionAlertType
	}{
		{"vector neighbor allows duplicate", EvolveOriginVectorNeighbor,
			[]model.EvolutionAlertType{model.EvolutionAlertTypeDuplicate, model.EvolutionAlertTypeConflict, model.EvolutionAlertTypeOutdated}},
		{"vector neighbor allows conflict", EvolveOriginVectorNeighbor,
			[]model.EvolutionAlertType{model.EvolutionAlertTypeDuplicate, model.EvolutionAlertTypeConflict, model.EvolutionAlertTypeOutdated}},
		{"vector neighbor allows outdated", EvolveOriginVectorNeighbor,
			[]model.EvolutionAlertType{model.EvolutionAlertTypeDuplicate, model.EvolutionAlertTypeConflict, model.EvolutionAlertTypeOutdated}},
		{"update relation allows outdated only", EvolveOriginUpdateRelation,
			[]model.EvolutionAlertType{model.EvolutionAlertTypeOutdated}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			allowed := evolutionAllowedTypes(tc.origin)
			var got []model.EvolutionAlertType
			for _, alertType := range []model.EvolutionAlertType{
				model.EvolutionAlertTypeDuplicate, model.EvolutionAlertTypeConflict, model.EvolutionAlertTypeOutdated,
			} {
				if allowed[alertType] {
					got = append(got, alertType)
				}
			}
			require.Equal(t, tc.want, got)
		})
	}
}
