package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"gorm.io/gorm"
)

// 依赖保持最小接口，便于服务层测试注入 fake。
// 对话与嵌入是两种不同的模型能力：判定关联必须用对话模型，向量召回必须用嵌入模型。
type relationChatResolver interface {
	ResolveChatModel(context.Context, uint) (*ChatCapability, error)
}

type relationEmbeddingResolver interface {
	ResolveEmbedding(context.Context, uint) (*ChatCapability, uint64, uint64, error)
}

type relationKnowledgeSource interface {
	Get(context.Context, uint, uint) (*model.KnowledgeItem, error)
	ListByIDs(context.Context, uint, []uint) ([]model.KnowledgeItem, error)
}

type relationPairStore interface {
	ListByKnowledge(context.Context, uint, uint) ([]repository.RelationRecord, error)
	CreateSuggestion(context.Context, *model.KnowledgeRelation) (bool, error)
}

type relationVectorSearcher interface {
	Search(context.Context, string, uint, []float64, int, float64) ([]VectorMatch, error)
}

type relationSuggester interface {
	Relate(context.Context, *ChatCapability, uint, uint, string, string, []RelateCandidate) ([]RelateRelation, error)
}

// RelationProcessor 执行 relation 任务：代表文本嵌入 → 向量召回 → 归并过滤 → AI 判定 → 落建议。
type RelationProcessor struct {
	Chat      relationChatResolver
	Embedding relationEmbeddingResolver
	Knowledge relationKnowledgeSource
	Relations relationPairStore
	Embedder  VectorEmbedder
	Vectors   relationVectorSearcher
	Client    relationSuggester
}

func (p *RelationProcessor) TaskType() string { return model.AiTaskTypeRelation }

func (p *RelationProcessor) Process(ctx context.Context, task *model.AiTask) (json.RawMessage, error) {
	if p == nil || p.Chat == nil || p.Embedding == nil || p.Knowledge == nil || p.Relations == nil ||
		p.Embedder == nil || p.Vectors == nil || p.Client == nil {
		return nil, TerminalTaskError("AI 关联未配置")
	}
	userID := task.UserID
	source, err := p.Knowledge.Get(ctx, userID, task.TargetID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, TerminalTaskError("知识不存在或已删除")
	}
	if err != nil {
		return nil, err
	}
	if source.ArchivedAt != nil {
		return nil, TerminalTaskError("知识已归档，无法生成关联建议")
	}
	if source.Status != model.KnowledgeReady {
		return nil, TerminalTaskError("知识尚未就绪，无法生成关联建议")
	}
	// 嵌入与判定都从同一份代表文本派生，但长度上限分开：嵌入只需要前 1000 字，
	// 判定侧可以拿到 /relate 契约允许的 2000 字。
	embeddingBody := relationSourceBody(source, RelationEmbeddingBodyMaxRunes)
	if embeddingBody == "" {
		return nil, TerminalTaskError("知识尚未就绪，无法生成关联建议")
	}
	// 标题一起计入长度预算：AI 侧对单条文本有上限，超长会让整批请求被 400 拒绝。
	embeddingText := truncateRunes(source.Title+"\n"+embeddingBody, RelationEmbeddingTextMaxRunes)

	// 模型配置先解析对话能力：它缺失时直接终止，不发起任何付费调用。
	chat, err := p.Chat.ResolveChatModel(ctx, userID)
	if err != nil {
		return nil, TerminalTaskError("未配置可用的对话模型，请前往模型设置")
	}
	snapshot, version, _, err := p.Embedding.ResolveEmbedding(ctx, userID)
	if err != nil {
		return nil, TerminalTaskError("未配置可用的 Embedding 模型，请前往模型设置")
	}
	embedCtx, cancel := context.WithTimeout(ctx, 110*time.Second)
	vectors, err := p.Embedder.Embed(embedCtx, snapshot, []string{embeddingText})
	cancel()
	if err != nil {
		// 400/403 表示文本或模型配置不合规：同一输入重试只会再次被拒，直接判终态。
		var rejected *EmbeddingRejectedError
		if errors.As(err, &rejected) {
			return nil, TerminalTaskError(rejected.Error())
		}
		return nil, err
	}
	if len(vectors) != 1 || len(vectors[0]) == 0 {
		return nil, errors.New("Embedding 服务返回了空向量，请稍后重试")
	}
	matches, err := p.Vectors.Search(ctx, vectorCollection(userID, version), userID, vectors[0], RelationVectorTopK, 0)
	if err != nil {
		return nil, err
	}

	// 一个知识有多个分块命中时只保留最高分，避免同一候选重复占用名额。
	best := make(map[uint]float64, len(matches))
	for _, match := range matches {
		if match.KnowledgeID == 0 || match.KnowledgeID == source.ID || match.Score < RelationMinScore {
			continue
		}
		if score, ok := best[match.KnowledgeID]; !ok || match.Score > score {
			best[match.KnowledgeID] = match.Score
		}
	}
	existing, err := p.Relations.ListByKnowledge(ctx, userID, source.ID)
	if err != nil {
		return nil, err
	}
	// 任意状态（含已拒绝）或任意类型的关系都抑制该对，防止覆盖用户决定。
	suppressed := make(map[uint]bool, len(existing))
	for _, row := range existing {
		if row.CounterpartID != 0 {
			suppressed[row.CounterpartID] = true
		}
	}
	type scored struct {
		id    uint
		score float64
	}
	ranked := make([]scored, 0, len(best))
	for id, score := range best {
		if !suppressed[id] {
			ranked = append(ranked, scored{id: id, score: score})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].id < ranked[j].id
		}
		return ranked[i].score > ranked[j].score
	})
	ids := make([]uint, 0, len(ranked))
	for _, candidate := range ranked {
		ids = append(ids, candidate.id)
	}
	details, err := p.Knowledge.ListByIDs(ctx, userID, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]model.KnowledgeItem, len(details))
	for _, item := range details {
		byID[item.ID] = item
	}
	candidates := make([]RelateCandidate, 0, RelationCandidateMax)
	selected := make(map[uint]bool, RelationCandidateMax)
	for _, candidate := range ranked {
		if len(candidates) == RelationCandidateMax {
			break
		}
		item, ok := byID[candidate.id]
		if !ok || item.ArchivedAt != nil {
			continue
		}
		candidates = append(candidates, RelateCandidate{
			KnowledgeID: item.ID,
			Title:       truncateRunes(strings.TrimSpace(item.Title), 200),
			Excerpt:     relationSourceBody(&item, RelationCandidateExcerptMaxRunes),
			Score:       candidate.score,
		})
		selected[item.ID] = true
	}
	if len(candidates) < 2 {
		return nil, TerminalTaskError("可用候选知识不足，暂时无法生成关联建议")
	}

	judgeExcerpt := relationSourceBody(source, RelationKnowledgeExcerptMaxRunes)
	relations, err := p.Client.Relate(ctx, chat, task.ID, source.ID, source.Title, judgeExcerpt, candidates)
	if err != nil {
		if errors.Is(err, ErrRelateInvalidOutput) || errors.Is(err, ErrRelateRejected) {
			return nil, TerminalTaskError(err.Error())
		}
		return nil, err
	}
	// 高置信度优先，保证同一对同类型只保留最强的一条建议；不同类型可以共存。
	sort.SliceStable(relations, func(i, j int) bool { return relations[i].Confidence > relations[j].Confidence })
	type suggestionPair struct {
		low, high    uint
		relationType model.RelationType
	}
	seen := make(map[suggestionPair]bool, RelationSuggestionMax)
	accepted := make([]RelateRelation, 0, RelationSuggestionMax)
	for _, relation := range relations {
		if len(accepted) == RelationSuggestionMax {
			break
		}
		if !relation.RelationType.Valid() || !selected[relation.TargetID] ||
			relation.Confidence < RelationConfidenceMin || relation.Confidence > 1 {
			continue
		}
		relation.Evidence.Reason = strings.TrimSpace(relation.Evidence.Reason)
		if relation.Evidence.Reason == "" {
			continue
		}
		low, high := source.ID, relation.TargetID
		if low > high {
			low, high = high, low
		}
		key := suggestionPair{low: low, high: high, relationType: relation.RelationType}
		if seen[key] {
			continue
		}
		seen[key] = true
		accepted = append(accepted, relation)
	}

	created := 0
	for _, relation := range accepted {
		evidence, err := json.Marshal(RelateEvidence{
			Reason:      relation.Evidence.Reason,
			SourceQuote: truncateRunes(strings.TrimSpace(relation.Evidence.SourceQuote), relationQuoteMaxRunes),
			TargetQuote: truncateRunes(strings.TrimSpace(relation.Evidence.TargetQuote), relationQuoteMaxRunes),
		})
		if err != nil {
			return nil, err
		}
		inserted, err := p.Relations.CreateSuggestion(ctx, &model.KnowledgeRelation{
			UserID:       userID,
			SourceID:     source.ID,
			TargetID:     relation.TargetID,
			RelationType: relation.RelationType,
			Status:       model.RelationSuggested,
			Source:       model.RelationSourceAI,
			Confidence:   relation.Confidence,
			Evidence:     string(evidence),
		})
		if err != nil {
			return nil, err
		}
		if inserted {
			created++
		}
	}
	payload, err := json.Marshal(RelationPayload{
		KnowledgeID:     source.ID,
		BaseVersion:     source.Version,
		SuggestionCount: created,
	})
	if err != nil {
		return nil, TerminalTaskError("关联结果序列化失败")
	}
	return payload, nil
}

// relationSourceBody 取代表文本：已采纳摘要优先，否则正文；按 rune 截断。
func relationSourceBody(item *model.KnowledgeItem, limit int) string {
	body := strings.TrimSpace(item.Summary)
	if body == "" {
		body = strings.TrimSpace(item.Content)
	}
	return truncateRunes(body, limit)
}
