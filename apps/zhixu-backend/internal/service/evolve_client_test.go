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
}
