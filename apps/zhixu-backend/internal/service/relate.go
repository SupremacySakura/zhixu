package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	"zhixu-backend/internal/model"
)

const (
	// RelationVectorTopK 是每次关联判定检索的分块数；归并到知识后最多取 10 条候选。
	RelationVectorTopK = 50
	// RelationMinScore 低于该向量相似度的候选不参与。
	RelationMinScore = 0.55
	// RelationCandidateMax 是发给模型的候选知识上限（与 issue 契约一致）。
	RelationCandidateMax = 10
	// RelationConfidenceMin 低于该置信度的关系建议直接丢弃。
	RelationConfidenceMin = 0.6
	// RelationSuggestionMax 是单次任务最多落库的建议条数。
	RelationSuggestionMax = 5
	// RelationKnowledgeExcerptMaxRunes 是送 /relate 判定的源知识摘录上限（契约 ≤2000），
	// 与 RelationEmbeddingBodyMaxRunes 分开：向量化不需要占用判定侧的全部长度。
	RelationKnowledgeExcerptMaxRunes = 2000
	// RelationCandidateExcerptMaxRunes 是候选知识发给模型的摘录上限。
	RelationCandidateExcerptMaxRunes = 1200
	// RelationEmbeddingBodyMaxRunes 是代表文本（无摘要时取正文）的嵌入上限。
	RelationEmbeddingBodyMaxRunes = 1000
	// RelationEmbeddingTextMaxRunes 是送给 /vectorize 的单条文本上限：标题（≤200 字，见
	// validateKnowledgeTitle）+ 换行 + 代表文本。标题必须一起计入预算，否则长中文标题会把
	// 文本顶到 AI 侧 schema 上限之上，整批请求被 400 拒绝——历史上「查找相关知识」正是这样必失败。
	RelationEmbeddingTextMaxRunes = RelationEmbeddingBodyMaxRunes + 201
	// relationQuoteMaxRunes 是证据引文的单侧长度上限。
	relationQuoteMaxRunes = 200
)

var (
	ErrRelateInvalidOutput = errors.New("AI 输出格式不符合要求")
	ErrRelateRejected      = errors.New("AI 关联请求被拒绝")
)

// RelationEnqueuePayload 是入队时记录的知识版本，供排查“建议基于哪个版本”。
type RelationEnqueuePayload struct {
	KnowledgeID uint `json:"knowledge_id"`
	BaseVersion uint `json:"base_version"`
}

// RelationPayload 是 relation 任务成功后写回 ai_tasks.payload 的产物。
type RelationPayload struct {
	KnowledgeID     uint `json:"knowledge_id"`
	BaseVersion     uint `json:"base_version"`
	SuggestionCount int  `json:"suggestion_count"`
}

// RelateCandidate 是请求中一条候选知识：标题、摘录与向量相似度。
type RelateCandidate struct {
	KnowledgeID uint    `json:"knowledge_id"`
	Title       string  `json:"title"`
	Excerpt     string  `json:"excerpt"`
	Score       float64 `json:"score"`
}

// RelateEvidence 是模型给出的判定理由与两侧原文引文。
type RelateEvidence struct {
	Reason      string `json:"reason"`
	SourceQuote string `json:"source_quote"`
	TargetQuote string `json:"target_quote"`
}

// RelateRelation 是模型返回的一条候选关系。
type RelateRelation struct {
	TargetID     uint               `json:"target_id"`
	RelationType model.RelationType `json:"relation_type"`
	Confidence   float64            `json:"confidence"`
	Evidence     RelateEvidence     `json:"evidence"`
}

// RelateClient 同步调用 AI 服务的 /relate；不自动重试，超时与 422 由调用方分类。
type RelateClient struct {
	AIURL  string
	Secret string
}

type relateRequest struct {
	Version     int               `json:"version"`
	Model       relateModel       `json:"model"`
	TaskID      uint              `json:"taskId"`
	KnowledgeID uint              `json:"knowledgeId"`
	Knowledge   relateKnowledge   `json:"knowledge"`
	Candidates  []RelateCandidate `json:"candidates"`
}

type relateModel struct {
	Protocol string `json:"protocol"`
	ModelID  string `json:"model_id"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
}

type relateKnowledge struct {
	Title   string `json:"title"`
	Excerpt string `json:"excerpt"`
}

type relateResponse struct {
	Relations []RelateRelation `json:"relations"`
}

// Relate 请求模型判定候选关系。返回前逐条清洗：非法类型、越界置信度、
// 空理由丢弃，引文截断到 200 字，最多保留 5 条。
func (c *RelateClient) Relate(ctx context.Context, snapshot *ChatCapability, taskID, knowledgeID uint, title, excerpt string, candidates []RelateCandidate) ([]RelateRelation, error) {
	if snapshot == nil {
		return nil, errors.New("AI 关联未配置")
	}
	if c == nil || c.Secret == "" {
		return nil, errors.New("INTERNAL_API_SECRET 未配置，无法调用 AI 服务")
	}
	title = strings.TrimSpace(title)
	excerpt = strings.TrimSpace(excerpt)
	if excerpt == "" {
		return nil, fmt.Errorf("%w: 知识摘要为空，无法判定关联", ErrRelateRejected)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: 候选知识不足", ErrRelateRejected)
	}
	req := relateRequest{
		Version: 1,
		Model: relateModel{
			Protocol: snapshot.Provider.Protocol,
			ModelID:  snapshot.Model.ModelID,
			BaseURL:  snapshot.Provider.BaseURL,
			APIKey:   snapshot.Key,
		},
		TaskID:      taskID,
		KnowledgeID: knowledgeID,
		Knowledge: relateKnowledge{
			Title:   truncateRunes(title, 200),
			Excerpt: truncateRunes(excerpt, RelationKnowledgeExcerptMaxRunes),
		},
	}
	for i, candidate := range candidates {
		if i == RelationCandidateMax {
			break
		}
		candidate.Title = truncateRunes(strings.TrimSpace(candidate.Title), 200)
		candidate.Excerpt = truncateRunes(strings.TrimSpace(candidate.Excerpt), RelationCandidateExcerptMaxRunes)
		req.Candidates = append(req.Candidates, candidate)
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("关联请求序列化失败: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.AIURL, "/")+"/relate", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("关联请求构造失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.Secret)
	client := &http.Client{
		Timeout: 100 * time.Second,
		Transport: &http.Transport{
			Proxy:                 nil,
			ResponseHeaderTimeout: 95 * time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	response, err := client.Do(httpReq)
	if err != nil {
		var netErr net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
			return nil, errors.New("AI 关联调用超时，请稍后重试")
		}
		return nil, errors.New("AI 服务未启动或无法连接，请检查 Mastra 服务")
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusUnprocessableEntity:
		return nil, ErrRelateInvalidOutput
	case http.StatusGatewayTimeout:
		return nil, errors.New("AI 关联调用超时，请稍后重试")
	case http.StatusBadRequest, http.StatusForbidden:
		return nil, fmt.Errorf("%w: 请检查 AI 服务与模型配置（HTTP %d）", ErrRelateRejected, response.StatusCode)
	default:
		return nil, fmt.Errorf("AI 关联服务暂不可用（HTTP %d）", response.StatusCode)
	}
	var result relateResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return nil, errors.New("AI 服务返回了无效的关联结果")
	}
	return sanitizeRelateRelations(result.Relations), nil
}

// sanitizeRelateRelations 丢弃单条非法输出，避免一条坏数据拖垮整批建议。
func sanitizeRelateRelations(relations []RelateRelation) []RelateRelation {
	out := make([]RelateRelation, 0, len(relations))
	for _, relation := range relations {
		relation.RelationType = model.RelationType(strings.TrimSpace(string(relation.RelationType)))
		relation.Evidence.Reason = strings.TrimSpace(relation.Evidence.Reason)
		relation.Evidence.SourceQuote = truncateRunes(strings.TrimSpace(relation.Evidence.SourceQuote), relationQuoteMaxRunes)
		relation.Evidence.TargetQuote = truncateRunes(strings.TrimSpace(relation.Evidence.TargetQuote), relationQuoteMaxRunes)
		if relation.TargetID == 0 || !relation.RelationType.Valid() ||
			math.IsNaN(relation.Confidence) || relation.Confidence < 0 || relation.Confidence > 1 ||
			relation.Evidence.Reason == "" {
			continue
		}
		out = append(out, relation)
		if len(out) == RelationSuggestionMax {
			break
		}
	}
	return out
}
