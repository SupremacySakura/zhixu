package service

import (
	"encoding/json"
	"testing"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"github.com/stretchr/testify/require"
)

// mergeAlert 造一条形状完整的提醒：载荷里带一条结论，版本与两侧 ID 可控。
// 结论与载荷顶层字段保持一致，模拟 buildAlert 产出的形状。
func mergeAlert(t *testing.T, alertType model.EvolutionAlertType, status model.EvolutionAlertStatus,
	triggerID, otherID uint, versions map[uint]uint, quote string) *model.EvolutionAlert {
	t.Helper()
	alert := &model.EvolutionAlert{
		UserID: 2, AlertType: alertType, Status: status, Severity: model.EvolutionSeverityStrong,
	}
	require.NoError(t, alert.SetSubjectIDs([]uint{triggerID, otherID}))
	require.NoError(t, alert.SetPairVersions(versions))
	evidence := []EvolutionEvidence{{KnowledgeID: triggerID, Quote: quote}}
	payload := EvolutionPayload{
		Type:            alertType,
		Reason:          "理由",
		Evidence:        evidence,
		SuggestedAction: evolveSuggestedAction(EvolutionPayload{Type: alertType}),
		Conclusions: []EvolutionConclusion{{
			Type:            alertType,
			TriggerID:       triggerID,
			Reason:          "理由",
			Evidence:        evidence,
			SuggestedAction: evolveSuggestedAction(EvolutionPayload{Type: alertType}),
			Severity:        model.EvolutionSeverityStrong,
		}},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	alert.Payload = string(body)
	alert.DedupeKey = EvolutionDedupeKey(alertType, []uint{triggerID, otherID}, evidence)
	return alert
}

// 知识对键与顺序无关：A 检查 B 与 B 检查 A 必须落在同一个键上。
func TestEvolutionPairKeyIgnoresOrder(t *testing.T) {
	require.Equal(t, EvolutionPairKey(10, 11), EvolutionPairKey(11, 10))
	require.NotEqual(t, EvolutionPairKey(10, 11), EvolutionPairKey(10, 12))
}

// 首次出现的知识对：直接新建。
func TestMergeEvolutionAlertCreatesWhenNoExisting(t *testing.T) {
	incoming := mergeAlert(t, model.EvolutionAlertTypeConflict, model.EvolutionAlertPending, 10, 11, map[uint]uint{10: 2, 11: 5}, "新值")

	alert, outcome := MergeEvolutionAlert(nil, incoming)

	require.Equal(t, repository.EvolutionAlertCreated, outcome)
	require.Equal(t, model.EvolutionAlertTypeConflict, alert.AlertType)
	require.Len(t, ParseEvolutionPayload(alert.Payload).Conclusions, 1)
}

// 同一证据再次命中：只刷新时间，结论不重复堆积。
func TestMergeEvolutionAlertRefreshesSameEvidence(t *testing.T) {
	existing := mergeAlert(t, model.EvolutionAlertTypeConflict, model.EvolutionAlertPending, 10, 11, map[uint]uint{10: 2, 11: 5}, "同一句")
	existing.ID = 7
	incoming := mergeAlert(t, model.EvolutionAlertTypeConflict, model.EvolutionAlertPending, 10, 11, map[uint]uint{10: 2, 11: 5}, "同一句")

	alert, outcome := MergeEvolutionAlert(existing, incoming)

	require.Equal(t, repository.EvolutionAlertRefreshed, outcome)
	require.Equal(t, uint(7), alert.ID, "合并应落在原行上")
	require.Len(t, ParseEvolutionPayload(alert.Payload).Conclusions, 1, "同一结论不该出现两次")
}

// 反向检查带来新结论：两条都保留，主结论按重复 → 冲突 → 过时 收敛。
func TestMergeEvolutionAlertKeepsBothDirectionsAndPicksPrimary(t *testing.T) {
	existing := mergeAlert(t, model.EvolutionAlertTypeOutdated, model.EvolutionAlertPending, 10, 11, map[uint]uint{10: 2, 11: 5}, "旧值")
	existing.ID = 7
	incoming := mergeAlert(t, model.EvolutionAlertTypeConflict, model.EvolutionAlertPending, 11, 10, map[uint]uint{10: 2, 11: 5}, "新值")

	alert, outcome := MergeEvolutionAlert(existing, incoming)

	payload := ParseEvolutionPayload(alert.Payload)
	require.Equal(t, repository.EvolutionAlertRefreshed, outcome)
	require.Len(t, payload.Conclusions, 2, "两个方向的结论都要留下")
	require.Equal(t, model.EvolutionAlertTypeConflict, alert.AlertType, "冲突优先于过时")
	require.Equal(t, model.EvolutionAlertTypeConflict, payload.Type, "载荷顶层类型要跟着主结论走")
}

// 已裁决且正文未变：保持抑制，不重新弹提醒。
func TestMergeEvolutionAlertSuppressesResolvedWithSameVersions(t *testing.T) {
	existing := mergeAlert(t, model.EvolutionAlertTypeConflict, model.EvolutionAlertResolved, 10, 11, map[uint]uint{10: 2, 11: 5}, "同一句")
	existing.ID = 7
	incoming := mergeAlert(t, model.EvolutionAlertTypeConflict, model.EvolutionAlertPending, 10, 11, map[uint]uint{10: 2, 11: 5}, "同一句")

	alert, outcome := MergeEvolutionAlert(existing, incoming)

	require.Equal(t, repository.EvolutionAlertSuppressed, outcome)
	require.Nil(t, alert, "抑制时不写库")
}

// 已裁决但正文变了：裁决针对的前提不成立，允许重新进入待办并留下历史。
func TestMergeEvolutionAlertRevivesResolvedAfterVersionChange(t *testing.T) {
	existing := mergeAlert(t, model.EvolutionAlertTypeConflict, model.EvolutionAlertResolved, 10, 11, map[uint]uint{10: 2, 11: 5}, "旧正文")
	existing.ID = 7
	existing.ResolvedAction = model.EvolutionActionKeepBoth
	existing.ResolutionNote = "先保留"
	incoming := mergeAlert(t, model.EvolutionAlertTypeConflict, model.EvolutionAlertPending, 10, 11, map[uint]uint{10: 3, 11: 5}, "新正文")

	alert, outcome := MergeEvolutionAlert(existing, incoming)

	require.Equal(t, repository.EvolutionAlertRevived, outcome)
	require.Equal(t, model.EvolutionAlertPending, alert.Status)
	require.Nil(t, alert.ResolvedAt)
	require.Empty(t, alert.ResolvedAction)
	require.Contains(t, alert.ResolutionNote, "先保留", "旧裁决备注要作为历史保留")
	require.Equal(t, uint(3), alert.PairVersionMap()[10], "版本要跟着更新")
}

// 暂不处理被再次命中：回到待办。
func TestMergeEvolutionAlertRevivesDeferred(t *testing.T) {
	existing := mergeAlert(t, model.EvolutionAlertTypeOutdated, model.EvolutionAlertDeferred, 10, 11, map[uint]uint{10: 2, 11: 5}, "旧值")
	existing.ID = 7
	incoming := mergeAlert(t, model.EvolutionAlertTypeOutdated, model.EvolutionAlertPending, 10, 11, map[uint]uint{10: 2, 11: 5}, "旧值")

	alert, outcome := MergeEvolutionAlert(existing, incoming)

	require.Equal(t, repository.EvolutionAlertRevived, outcome)
	require.Equal(t, model.EvolutionAlertPending, alert.Status)
}
