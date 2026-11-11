package service

import (
	"context"
	"testing"
	"time"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type graphFakeStore struct {
	node             *repository.GraphNodeRow
	nodeErr          error
	recent           []repository.GraphNodeRow
	recentLimit      int
	nodes            map[uint]repository.GraphNodeRow
	nodeIDs          []uint
	relations        []model.KnowledgeRelation
	relationIDs      []uint
	includeSuggested bool
}

func (f *graphFakeStore) GetNode(_ context.Context, _ uint, id uint) (*repository.GraphNodeRow, error) {
	if f.nodeErr != nil {
		return nil, f.nodeErr
	}
	if f.node != nil && f.node.ID == id {
		return f.node, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *graphFakeStore) ListRecentNodes(_ context.Context, _ uint, limit int) ([]repository.GraphNodeRow, error) {
	f.recentLimit = limit
	return f.recent, nil
}

func (f *graphFakeStore) ListNodesByIDs(_ context.Context, _ uint, ids []uint) ([]repository.GraphNodeRow, error) {
	f.nodeIDs = ids
	rows := make([]repository.GraphNodeRow, 0, len(ids))
	for _, id := range ids {
		if row, ok := f.nodes[id]; ok {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (f *graphFakeStore) ListRelations(_ context.Context, _ uint, ids []uint, includeSuggested bool) ([]model.KnowledgeRelation, error) {
	f.relationIDs = ids
	f.includeSuggested = includeSuggested
	return f.relations, nil
}

func graphRow(id uint, title, sourceType string, updated time.Time) repository.GraphNodeRow {
	return repository.GraphNodeRow{ID: id, Title: title, SourceType: sourceType, Summary: title + "摘要", UpdatedAt: updated}
}

func graphNodeByID(t *testing.T, view GraphView, id uint) GraphNode {
	t.Helper()
	for _, node := range view.Nodes {
		if node.ID == id {
			return node
		}
	}
	t.Fatalf("节点 %d 不在图谱中", id)
	return GraphNode{}
}

func graphEdgeExists(view GraphView, sourceID, targetID uint) bool {
	for _, edge := range view.Edges {
		if edge.SourceID == sourceID && edge.TargetID == targetID {
			return true
		}
	}
	return false
}

var graphBaseTime = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

// 无中心时取最近 50 条未归档知识，只保留两端都在子图内的已确认关系。
func TestGraphWithoutCenterBuildsRecentSubgraph(t *testing.T) {
	store := &graphFakeStore{
		recent: []repository.GraphNodeRow{
			graphRow(1, "知识一", "manual", graphBaseTime.Add(2*time.Hour)),
			graphRow(2, "知识二", "web", graphBaseTime.Add(time.Hour)),
			graphRow(3, "知识三", "markdown", graphBaseTime),
		},
		nodes: map[uint]repository.GraphNodeRow{
			1: graphRow(1, "知识一", "manual", graphBaseTime.Add(2*time.Hour)),
			2: graphRow(2, "知识二", "web", graphBaseTime.Add(time.Hour)),
			3: graphRow(3, "知识三", "markdown", graphBaseTime),
		},
		relations: []model.KnowledgeRelation{
			{ID: 11, UserID: 2, SourceID: 1, TargetID: 2, RelationType: model.RelationTypeRelated, Status: model.RelationConfirmed, Source: model.RelationSourceManual},
			{ID: 12, UserID: 2, SourceID: 2, TargetID: 3, RelationType: model.RelationTypeDependsOn, Status: model.RelationConfirmed, Source: model.RelationSourceAI, Confidence: 0.7},
			// 对端不在最近 50 条内：必须丢弃。
			{ID: 13, UserID: 2, SourceID: 1, TargetID: 99, RelationType: model.RelationTypeRelated, Status: model.RelationConfirmed, Source: model.RelationSourceAI},
		},
	}

	view, err := NewGraphService(store).Graph(context.Background(), 2, GraphQuery{})
	require.NoError(t, err)
	require.Equal(t, 50, store.recentLimit)
	require.ElementsMatch(t, []uint{1, 2, 3}, store.relationIDs)
	require.False(t, view.Truncated)
	require.Len(t, view.Nodes, 3)
	require.Len(t, view.Edges, 2)
	require.True(t, graphEdgeExists(view, 1, 2))
	require.True(t, graphEdgeExists(view, 2, 3))
	require.Equal(t, 1, graphNodeByID(t, view, 1).Degree)
	require.Equal(t, 2, graphNodeByID(t, view, 2).Degree)
	require.Equal(t, 1, graphNodeByID(t, view, 3).Degree)
	require.Equal(t, "web", graphNodeByID(t, view, 2).SourceType)
	require.Equal(t, "知识二摘要", graphNodeByID(t, view, 2).Summary)
}

// 有中心时返回中心 + 一跳邻居，方向保留 source_id→target_id，归档对端被丢弃。
func TestGraphWithCenterReturnsOneHopNeighbors(t *testing.T) {
	center := graphRow(5, "中心知识", "pdf", graphBaseTime)
	store := &graphFakeStore{
		node:  &center,
		nodes: map[uint]repository.GraphNodeRow{5: center, 6: graphRow(6, "邻居六", "manual", graphBaseTime), 7: graphRow(7, "邻居七", "web", graphBaseTime)},
		relations: []model.KnowledgeRelation{
			{ID: 21, SourceID: 7, TargetID: 5, RelationType: model.RelationTypeDependsOn, Status: model.RelationConfirmed, Source: model.RelationSourceManual},
			{ID: 22, SourceID: 5, TargetID: 6, RelationType: model.RelationTypeRelated, Status: model.RelationConfirmed, Source: model.RelationSourceAI, Confidence: 0.8},
			// 对端 8 已被归档（仓储会过滤），即使出现也不能进入图谱。
			{ID: 23, SourceID: 5, TargetID: 8, RelationType: model.RelationTypeSupplements, Status: model.RelationConfirmed, Source: model.RelationSourceAI},
		},
	}

	view, err := NewGraphService(store).Graph(context.Background(), 2, GraphQuery{CenterID: 5})
	require.NoError(t, err)
	require.Equal(t, []uint{5}, store.relationIDs)
	require.ElementsMatch(t, []uint{6, 7, 8}, store.nodeIDs)
	require.False(t, view.Truncated)
	require.Len(t, view.Nodes, 3)
	require.Equal(t, uint(5), view.Nodes[0].ID, "中心节点排在首位")
	require.Equal(t, 2, graphNodeByID(t, view, 5).Degree)
	require.Len(t, view.Edges, 2)
	require.True(t, graphEdgeExists(view, 7, 5))
	require.True(t, graphEdgeExists(view, 5, 6))
}

func TestGraphIncludeSuggestedOnlyWhenAsked(t *testing.T) {
	center := graphRow(5, "中心知识", "manual", graphBaseTime)
	relations := []model.KnowledgeRelation{
		{ID: 31, SourceID: 5, TargetID: 6, RelationType: model.RelationTypeRelated, Status: model.RelationConfirmed, Source: model.RelationSourceManual},
		{ID: 32, SourceID: 5, TargetID: 7, RelationType: model.RelationTypeRelated, Status: model.RelationSuggested, Source: model.RelationSourceAI, Confidence: 0.66},
	}

	t.Run("default excludes suggested", func(t *testing.T) {
		store := &graphFakeStore{
			node:      &center,
			nodes:     map[uint]repository.GraphNodeRow{5: center, 6: graphRow(6, "六", "text", graphBaseTime), 7: graphRow(7, "七", "text", graphBaseTime)},
			relations: relations,
		}
		view, err := NewGraphService(store).Graph(context.Background(), 2, GraphQuery{CenterID: 5})
		require.NoError(t, err)
		require.False(t, store.includeSuggested)
		require.Len(t, view.Nodes, 2)
		require.Len(t, view.Edges, 1)
	})

	t.Run("flag includes suggested", func(t *testing.T) {
		store := &graphFakeStore{
			node:      &center,
			nodes:     map[uint]repository.GraphNodeRow{5: center, 6: graphRow(6, "六", "text", graphBaseTime), 7: graphRow(7, "七", "text", graphBaseTime)},
			relations: relations,
		}
		view, err := NewGraphService(store).Graph(context.Background(), 2, GraphQuery{CenterID: 5, IncludeSuggested: true})
		require.NoError(t, err)
		require.True(t, store.includeSuggested)
		require.Len(t, view.Nodes, 3)
		require.Len(t, view.Edges, 2)
		require.Equal(t, model.RelationSuggested, view.Edges[1].Status)
		require.Equal(t, 0.66, view.Edges[1].Confidence)
	})
}

// 中心不存在或已归档：仓储统一返回 gorm.ErrRecordNotFound，服务映射为知识不存在。
func TestGraphCenterMissingMapsToKnowledgeNotFound(t *testing.T) {
	store := &graphFakeStore{nodeErr: gorm.ErrRecordNotFound}
	_, err := NewGraphService(store).Graph(context.Background(), 2, GraphQuery{CenterID: 5})
	require.ErrorIs(t, err, ErrKnowledgeNotFound)
	require.Nil(t, store.relationIDs)
}

// 节点超过 100 时按度数截断并标记 truncated；中心节点始终保留。
func TestGraphTruncatesToHundredNodes(t *testing.T) {
	t.Run("without center keeps highest degree", func(t *testing.T) {
		rows := make([]repository.GraphNodeRow, 0, 120)
		for id := uint(1); id <= 120; id++ {
			rows = append(rows, graphRow(id, "知识", "manual", graphBaseTime))
		}
		relations := make([]model.KnowledgeRelation, 0, 20)
		for id := uint(2); id <= 21; id++ {
			relations = append(relations, model.KnowledgeRelation{ID: 100 + id, SourceID: 1, TargetID: id, RelationType: model.RelationTypeRelated, Status: model.RelationConfirmed, Source: model.RelationSourceManual})
		}
		store := &graphFakeStore{recent: rows, relations: relations}

		view, err := NewGraphService(store).Graph(context.Background(), 2, GraphQuery{})
		require.NoError(t, err)
		require.True(t, view.Truncated)
		require.Len(t, view.Nodes, 100)
		require.Equal(t, 20, graphNodeByID(t, view, 1).Degree)
		require.NotContains(t, graphNodeIDs(view), uint(120))
		require.Contains(t, graphNodeIDs(view), uint(100))
		require.Len(t, view.Edges, 20)
	})

	t.Run("with center keeps center", func(t *testing.T) {
		center := graphRow(500, "中心知识", "manual", graphBaseTime)
		nodes := map[uint]repository.GraphNodeRow{500: center}
		relations := make([]model.KnowledgeRelation, 0, 120)
		for id := uint(1); id <= 120; id++ {
			nodes[id] = graphRow(id, "邻居", "manual", graphBaseTime)
			relations = append(relations, model.KnowledgeRelation{ID: 200 + id, SourceID: 500, TargetID: id, RelationType: model.RelationTypeRelated, Status: model.RelationConfirmed, Source: model.RelationSourceManual})
		}
		store := &graphFakeStore{node: &center, nodes: nodes, relations: relations}

		view, err := NewGraphService(store).Graph(context.Background(), 2, GraphQuery{CenterID: 500})
		require.NoError(t, err)
		require.True(t, view.Truncated)
		require.Len(t, view.Nodes, 100)
		require.Equal(t, uint(500), view.Nodes[0].ID)
		require.Len(t, view.Edges, 99)
		// 被截掉的 21 个邻居不再计入中心度数，节点大小与实际边一致。
		require.Equal(t, 99, graphNodeByID(t, view, 500).Degree)
	})
}

func graphNodeIDs(view GraphView) []uint {
	ids := make([]uint, 0, len(view.Nodes))
	for _, node := range view.Nodes {
		ids = append(ids, node.ID)
	}
	return ids
}
