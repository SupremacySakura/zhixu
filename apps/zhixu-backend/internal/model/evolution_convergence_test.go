package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 知识对键与顺序无关：A 检查 B 与 B 检查 A 必须落在同一行上。
func TestEvolutionPairKeyIgnoresOrder(t *testing.T) {
	require.Equal(t, EvolutionPairKey(10, 11), EvolutionPairKey(11, 10))
	require.NotEqual(t, EvolutionPairKey(10, 11), EvolutionPairKey(10, 12))
}

// 结论集合的第 0 条必须就是主结论。
//
// 收敛规则住在 model 层，前端与评测都按 conclusions[0] 取主结论，因此这条契约
// 必须在这里直接锁住，不能只靠上层调用方的测试间接保证。
func TestMergeEvolutionConclusionsPutsPrimaryFirst(t *testing.T) {
	outdated := EvolutionConclusion{
		Type: EvolutionAlertTypeOutdated, TriggerID: 10, Origin: EvolutionOriginUpdateRelation,
	}
	duplicate := EvolutionConclusion{Type: EvolutionAlertTypeDuplicate, TriggerID: 11}

	merged := MergeEvolutionConclusions([]EvolutionConclusion{duplicate}, []EvolutionConclusion{outdated})

	require.Len(t, merged, 2)
	require.Equal(t, EvolutionAlertTypeOutdated, merged[0].Type,
		"用户声明的更新关系过时优先于模型猜出的重复")
	require.Equal(t, PrimaryConclusion(merged), merged[0], "第 0 条必须等于主结论")
}

// 历史载荷只有顶层字段时，结论集合要用顶层字段合成一条，不能当成「没有结论」。
func TestConclusionsOfFallsBackToTopLevelFields(t *testing.T) {
	payload := EvolutionPayload{
		Type:     EvolutionAlertTypeConflict,
		Reason:   "两条结论互斥",
		Evidence: []EvolutionEvidence{{KnowledgeID: 10, Quote: "甲"}},
	}

	conclusions := ConclusionsOf(payload)

	require.Len(t, conclusions, 1)
	require.Equal(t, EvolutionAlertTypeConflict, conclusions[0].Type)
	require.Equal(t, "两条结论互斥", conclusions[0].Reason)
}
