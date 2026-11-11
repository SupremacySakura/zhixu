package service

import (
	"context"
	"sort"
	"time"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"
)

const (
	// graphRecentLimit 是无中心时默认子图的节点数（最近更新的未归档知识）。
	graphRecentLimit = 50
	// graphNodeLimit 是单次图谱响应的节点上限，超出按连接度截断。
	graphNodeLimit = 100
)

// GraphQuery 是局部图谱查询参数；深度固定为 1，由 handler 校验。
// TagID 与 SourceType 先裁剪节点集合，再取两端都在集合内的关系；两者同时给出时取交集。
type GraphQuery struct {
	CenterID         uint
	IncludeSuggested bool
	TagID            uint
	SourceType       string
}

// GraphNode 是图谱节点视图，degree 只统计本次响应内的边。
type GraphNode struct {
	ID         uint      `json:"id"`
	Title      string    `json:"title"`
	SourceType string    `json:"source_type"`
	Summary    string    `json:"summary"`
	Degree     int       `json:"degree"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// GraphEdge 是图谱边视图，方向保持关系表的 source_id→target_id。
type GraphEdge struct {
	ID           uint                 `json:"id"`
	SourceID     uint                 `json:"source_id"`
	TargetID     uint                 `json:"target_id"`
	RelationType model.RelationType   `json:"relation_type"`
	Status       model.RelationStatus `json:"status"`
	Source       model.RelationSource `json:"source"`
	Confidence   float64              `json:"confidence"`
}

// GraphView 是图谱接口响应；nodes/edges 始终为数组，前端无需处理 null。
type GraphView struct {
	Nodes     []GraphNode `json:"nodes"`
	Edges     []GraphEdge `json:"edges"`
	Truncated bool        `json:"truncated"`
}

// GraphStore 是图谱读取的最小仓储能力（*repository.GraphRepository 满足）。
type GraphStore interface {
	GetNode(context.Context, uint, uint) (*repository.GraphNodeRow, error)
	ListRecentNodes(context.Context, uint, repository.GraphNodeFilter, int) ([]repository.GraphNodeRow, error)
	ListNodesByIDs(context.Context, uint, repository.GraphNodeFilter, []uint) ([]repository.GraphNodeRow, error)
	ListRelations(context.Context, uint, []uint, bool) ([]model.KnowledgeRelation, error)
	ListRelationsAmong(context.Context, uint, []uint, bool) ([]model.KnowledgeRelation, error)
}

type GraphService struct{ store GraphStore }

func NewGraphService(store GraphStore) *GraphService { return &GraphService{store: store} }

// Graph 组装局部图谱：无中心时取最近 50 条知识，有中心时取中心与一跳邻居。
// 关系只保留两端都在候选节点内的行，节点超过 100 时按连接度截断。
func (s *GraphService) Graph(ctx context.Context, userID uint, query GraphQuery) (GraphView, error) {
	if !validGraphSourceType(query.SourceType) {
		// source_type 只接受 manual/web；其他值按「无匹配节点」处理，返回空图而不是 4xx/5xx。
		return emptyGraphView(), nil
	}
	if query.CenterID == 0 {
		return s.recentGraph(ctx, userID, query)
	}
	return s.centerGraph(ctx, userID, query)
}

func validGraphSourceType(sourceType string) bool {
	switch sourceType {
	case "", model.KnowledgeSourceTypeManual, model.KnowledgeSourceTypeWeb:
		return true
	default:
		return false
	}
}

func graphFilter(query GraphQuery) repository.GraphNodeFilter {
	return repository.GraphNodeFilter{TagID: query.TagID, SourceType: query.SourceType}
}

func (s *GraphService) recentGraph(ctx context.Context, userID uint, query GraphQuery) (GraphView, error) {
	rows, err := s.store.ListRecentNodes(ctx, userID, graphFilter(query), graphRecentLimit)
	if err != nil {
		return emptyGraphView(), err
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	if len(ids) == 0 {
		return emptyGraphView(), nil
	}
	relations, err := s.store.ListRelationsAmong(ctx, userID, ids, query.IncludeSuggested)
	if err != nil {
		return emptyGraphView(), err
	}
	return buildGraphView(rows, filterGraphRelations(relations, query.IncludeSuggested), 0), nil
}

// centerGraph 读取中心与其直接关系，再批量取对端节点；中心不存在/归档/属于他人时
// 仓储返回 gorm.ErrRecordNotFound，这里统一映射为知识不存在（HTTP 404）。
// 中心本身也要通过筛选：不满足时返回空图 200（而不是 404），符合「筛选是节点集合条件」的语义。
func (s *GraphService) centerGraph(ctx context.Context, userID uint, query GraphQuery) (GraphView, error) {
	center, err := s.store.GetNode(ctx, userID, query.CenterID)
	if err != nil {
		return emptyGraphView(), knowledgeError(err)
	}
	relations, err := s.store.ListRelations(ctx, userID, []uint{center.ID}, query.IncludeSuggested)
	if err != nil {
		return emptyGraphView(), err
	}
	relations = filterGraphRelations(relations, query.IncludeSuggested)
	candidateIDs := make([]uint, 1, len(relations)+1)
	candidateIDs[0] = center.ID
	seen := map[uint]bool{center.ID: true}
	for _, relation := range relations {
		var counterpart uint
		switch center.ID {
		case relation.SourceID:
			counterpart = relation.TargetID
		case relation.TargetID:
			counterpart = relation.SourceID
		default:
			continue
		}
		if seen[counterpart] {
			continue
		}
		seen[counterpart] = true
		candidateIDs = append(candidateIDs, counterpart)
	}
	rows, err := s.store.ListNodesByIDs(ctx, userID, graphFilter(query), candidateIDs)
	if err != nil {
		return emptyGraphView(), err
	}
	if !graphRowsContain(rows, center.ID) {
		return emptyGraphView(), nil
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	among, err := s.store.ListRelationsAmong(ctx, userID, ids, query.IncludeSuggested)
	if err != nil {
		return emptyGraphView(), err
	}
	return buildGraphView(rows, filterGraphRelations(among, query.IncludeSuggested), center.ID), nil
}

func graphRowsContain(rows []repository.GraphNodeRow, id uint) bool {
	for _, row := range rows {
		if row.ID == id {
			return true
		}
	}
	return false
}

// filterGraphRelations 兜底裁剪状态：rejected 永不进入图谱，suggested 仅在显式开启时保留。
// 仓储已按同样条件过滤，这里避免错误实现的上游把建议边泄漏到默认视图。
func filterGraphRelations(relations []model.KnowledgeRelation, includeSuggested bool) []model.KnowledgeRelation {
	filtered := make([]model.KnowledgeRelation, 0, len(relations))
	for _, relation := range relations {
		if relation.Status == model.RelationRejected {
			continue
		}
		if relation.Status == model.RelationSuggested && !includeSuggested {
			continue
		}
		filtered = append(filtered, relation)
	}
	return filtered
}

func emptyGraphView() GraphView {
	return GraphView{Nodes: []GraphNode{}, Edges: []GraphEdge{}}
}

func buildGraphView(rows []repository.GraphNodeRow, relations []model.KnowledgeRelation, centerID uint) GraphView {
	nodeMap := make(map[uint]repository.GraphNodeRow, len(rows))
	for _, row := range rows {
		nodeMap[row.ID] = row
	}
	degree := make(map[uint]int, len(rows))
	edges := make([]GraphEdge, 0, len(relations))
	for _, relation := range relations {
		if _, ok := nodeMap[relation.SourceID]; !ok {
			continue
		}
		if _, ok := nodeMap[relation.TargetID]; !ok {
			continue
		}
		edges = append(edges, GraphEdge{
			ID:           relation.ID,
			SourceID:     relation.SourceID,
			TargetID:     relation.TargetID,
			RelationType: relation.RelationType,
			Status:       relation.Status,
			Source:       relation.Source,
			Confidence:   relation.Confidence,
		})
		degree[relation.SourceID]++
		degree[relation.TargetID]++
	}

	nodes := make([]GraphNode, 0, len(rows))
	for _, row := range rows {
		nodes = append(nodes, GraphNode{
			ID: row.ID, Title: row.Title, SourceType: row.SourceType, Summary: row.Summary,
			Degree: degree[row.ID], UpdatedAt: row.UpdatedAt,
		})
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Degree != nodes[j].Degree {
			return nodes[i].Degree > nodes[j].Degree
		}
		if !nodes[i].UpdatedAt.Equal(nodes[j].UpdatedAt) {
			return nodes[i].UpdatedAt.After(nodes[j].UpdatedAt)
		}
		return nodes[i].ID < nodes[j].ID
	})
	if centerID != 0 {
		// 中心节点固定在首位：即使其度数在截断线上也必须保留。
		for i := range nodes {
			if nodes[i].ID == centerID {
				nodes[0], nodes[i] = nodes[i], nodes[0]
				break
			}
		}
	}

	truncated := len(nodes) > graphNodeLimit
	if truncated {
		nodes = nodes[:graphNodeLimit]
	}
	kept := make(map[uint]bool, len(nodes))
	for _, node := range nodes {
		kept[node.ID] = true
	}
	visibleEdges := make([]GraphEdge, 0, len(edges))
	for _, edge := range edges {
		if kept[edge.SourceID] && kept[edge.TargetID] {
			visibleEdges = append(visibleEdges, edge)
		}
	}
	// 截断后重新计算度数，确保节点大小只反映实际返回的边，而不是被裁掉邻居的边。
	visibleDegree := make(map[uint]int, len(nodes))
	for _, edge := range visibleEdges {
		visibleDegree[edge.SourceID]++
		visibleDegree[edge.TargetID]++
	}
	for i := range nodes {
		nodes[i].Degree = visibleDegree[nodes[i].ID]
	}
	return GraphView{Nodes: nodes, Edges: visibleEdges, Truncated: truncated}
}
