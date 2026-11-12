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
	"unicode/utf8"

	"zhixu-backend/internal/model"
)

// 候选来源：updates 关系是用户显式声明过的强信号，向量近邻只是相似度提示。
const (
	EvolveOriginUpdateRelation = "update_relation"
	EvolveOriginVectorNeighbor = "vector_neighbor"
)

var (
	// ErrEvolveInvalidOutput 表示模型输出无法解析成判定结构：重试只会再被拒，任务判终态。
	ErrEvolveInvalidOutput = errors.New("AI 演化判定输出格式不符合要求")
	// ErrEvolveRejected 表示请求本身被拒绝（模型地址或凭证不合规）：同样不可重试。
	ErrEvolveRejected = errors.New("AI 演化判定请求被拒绝")
)

// EvolveCandidate 是送给模型的一条候选知识。
type EvolveCandidate struct {
	KnowledgeID uint
	Title       string
	Excerpt     string
	// Score 是该候选与触发知识的向量相似度；来自 updates 关系的候选没有相似度，留 0。
	Score  float64
	Origin string
}

// EvolveJudgement 是模型对一条候选给出的结论。
type EvolveJudgement struct {
	TargetID uint
	Type     model.EvolutionAlertType
	Reason   string
	Evidence []EvolutionEvidence
	Severity model.EvolutionAlertSeverity
	// 冲突场景的修订稿；重复与过时留空。
	RevisedTitle   string
	RevisedContent string
}

type evolveRequest struct {
	Version     int               `json:"version"`
	Model       evolveModel       `json:"model"`
	TaskID      uint              `json:"taskId"`
	KnowledgeID uint              `json:"knowledgeId"`
	Knowledge   evolveKnowledge   `json:"knowledge"`
	Candidates  []evolveCandidate `json:"candidates"`
}

type evolveModel struct {
	Protocol string `json:"protocol"`
	ModelID  string `json:"model_id"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
}

type evolveKnowledge struct {
	Title   string `json:"title"`
	Excerpt string `json:"excerpt"`
}

type evolveCandidate struct {
	KnowledgeID uint    `json:"knowledge_id"`
	Title       string  `json:"title"`
	Excerpt     string  `json:"excerpt"`
	Score       float64 `json:"score"`
	Origin      string  `json:"origin"`
}

type evolveResponse struct {
	Judgements []struct {
		TargetID uint   `json:"target_id"`
		Type     string `json:"type"`
		Reason   string `json:"reason"`
		Evidence []struct {
			KnowledgeID uint   `json:"knowledge_id"`
			Quote       string `json:"quote"`
		} `json:"evidence"`
		Severity       string `json:"severity"`
		RevisedTitle   string `json:"revised_title"`
		RevisedContent string `json:"revised_content"`
	} `json:"judgements"`
}

// EvolveClient 同步调用 AI 服务的 /evolve；不自动重试，超时与 422 由调用方分类。
type EvolveClient struct {
	AIURL  string
	Secret string
}

// Evolve 请求模型一次性判定重复、过时与冲突。返回前逐条清洗：
// 未知类型、越界目标、空理由丢弃，引文截断到 200 字，最多保留 5 条。
func (c *EvolveClient) Evolve(ctx context.Context, snapshot *ChatCapability, taskID, knowledgeID uint, title, excerpt string, candidates []EvolveCandidate) ([]EvolveJudgement, error) {
	if snapshot == nil {
		return nil, errors.New("AI 演化判定未配置")
	}
	if c == nil || c.Secret == "" {
		return nil, errors.New("INTERNAL_API_SECRET 未配置，无法调用 AI 服务")
	}
	excerpt = strings.TrimSpace(excerpt)
	if excerpt == "" {
		return nil, fmt.Errorf("%w: 知识摘录为空，无法判定演化关系", ErrEvolveRejected)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: 候选知识不足", ErrEvolveRejected)
	}
	req := evolveRequest{
		Version: 1,
		Model: evolveModel{
			Protocol: snapshot.Provider.Protocol,
			ModelID:  snapshot.Model.ModelID,
			BaseURL:  snapshot.Provider.BaseURL,
			APIKey:   snapshot.Key,
		},
		TaskID:      taskID,
		KnowledgeID: knowledgeID,
		Knowledge: evolveKnowledge{
			Title:   truncateRunes(title, 200),
			Excerpt: truncateRunes(excerpt, RelationKnowledgeExcerptMaxRunes),
		},
	}
	for i, candidate := range candidates {
		if i == EvolveCandidateMax {
			break
		}
		req.Candidates = append(req.Candidates, evolveCandidate{
			KnowledgeID: candidate.KnowledgeID,
			Title:       truncateRunes(strings.TrimSpace(candidate.Title), 200),
			Excerpt:     truncateRunes(strings.TrimSpace(candidate.Excerpt), EvolveExcerptMaxRunes),
			Score:       candidate.Score,
			Origin:      candidate.Origin,
		})
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("演化判定请求序列化失败: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.AIURL, "/")+"/evolve", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("演化判定请求构造失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.Secret)
	// 比 /relate 宽松：一次要判三类结论，冲突场景还要生成修订稿。
	client := &http.Client{
		Timeout: 125 * time.Second,
		Transport: &http.Transport{
			Proxy:                 nil,
			ResponseHeaderTimeout: 120 * time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	response, err := client.Do(httpReq)
	if err != nil {
		var netErr net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
			return nil, errors.New("AI 演化判定调用超时，请稍后重试")
		}
		return nil, errors.New("AI 服务未启动或无法连接，请检查 Mastra 服务")
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusUnprocessableEntity:
		return nil, ErrEvolveInvalidOutput
	case http.StatusGatewayTimeout:
		return nil, errors.New("AI 演化判定调用超时，请稍后重试")
	case http.StatusBadRequest, http.StatusForbidden:
		return nil, fmt.Errorf("%w: 请检查 AI 服务与模型配置（HTTP %d）", ErrEvolveRejected, response.StatusCode)
	default:
		return nil, fmt.Errorf("AI 演化判定服务暂不可用（HTTP %d）", response.StatusCode)
	}
	var result evolveResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return nil, errors.New("AI 服务返回了无效的演化判定结果")
	}
	return sanitizeEvolveJudgements(knowledgeID, candidates, result.Judgements), nil
}

type rawEvolveJudgement = struct {
	TargetID uint   `json:"target_id"`
	Type     string `json:"type"`
	Reason   string `json:"reason"`
	Evidence []struct {
		KnowledgeID uint   `json:"knowledge_id"`
		Quote       string `json:"quote"`
	} `json:"evidence"`
	Severity       string `json:"severity"`
	RevisedTitle   string `json:"revised_title"`
	RevisedContent string `json:"revised_content"`
}

// sanitizeEvolveJudgements 丢弃单条非法输出，避免一条坏数据拖垮整批判定。
// 目标必须是本批候选之一，证据的知识 ID 只能是触发知识或该候选——模型偶尔会引用批次外的 ID。
func sanitizeEvolveJudgements(triggerID uint, candidates []EvolveCandidate, raw []rawEvolveJudgement) []EvolveJudgement {
	allowed := make(map[uint]bool, len(candidates))
	for _, candidate := range candidates {
		allowed[candidate.KnowledgeID] = true
	}
	out := make([]EvolveJudgement, 0, len(raw))
	for _, entry := range raw {
		alertType := model.EvolutionAlertType(strings.TrimSpace(entry.Type))
		reason := truncateRunes(strings.TrimSpace(entry.Reason), EvolveQuoteMaxRunes)
		if entry.TargetID == 0 || !allowed[entry.TargetID] || !alertType.Valid() || reason == "" {
			continue
		}
		evidence := make([]EvolutionEvidence, 0, len(entry.Evidence))
		for _, item := range entry.Evidence {
			quote := truncateRunes(strings.TrimSpace(item.Quote), EvolveQuoteMaxRunes)
			if quote == "" {
				continue
			}
			// 证据只认参与本次判定的两侧，别让模型把第三条知识的话塞进来。
			if item.KnowledgeID != triggerID && item.KnowledgeID != entry.TargetID {
				continue
			}
			evidence = append(evidence, EvolutionEvidence{KnowledgeID: item.KnowledgeID, Quote: quote})
		}
		if len(evidence) == 0 {
			continue
		}
		severity := model.EvolutionAlertSeverity(strings.TrimSpace(entry.Severity))
		if !severity.Valid() {
			severity = model.EvolutionSeverityReview
		}
		// 修订稿只保留完整的一份：超长就整份丢弃而不是截断，半份修订稿比没有更糟，
		// 前端在修订稿缺失时会把「采纳修订」置灰。
		title := truncateRunes(strings.TrimSpace(entry.RevisedTitle), 200)
		content := strings.TrimSpace(entry.RevisedContent)
		if title == "" || content == "" || utf8.RuneCountInString(content) > EvolveRevisedContentMaxRunes {
			title, content = "", ""
		}
		out = append(out, EvolveJudgement{
			TargetID:       entry.TargetID,
			Type:           alertType,
			Reason:         reason,
			Evidence:       evidence,
			Severity:       severity,
			RevisedTitle:   title,
			RevisedContent: content,
		})
		if len(out) == EvolveAlertMaxPerTask {
			break
		}
	}
	return out
}

// evolutionAllowedTypes 按候选来源与相似度分派允许的结论类型。
//
// updates 关系是用户显式声明过的「谁更新谁」，这类候选只接受过时结论——把它判成重复
// 会给出「归档其一」这种明显错误的选择。向量近邻只能按相似度分派：越像越可能重复，
// 中等相似才可能是对同一事实的不同说法（冲突），再低就没有依据；过时看的是「谁更晚更全」，
// 与相似度高低无关，因此从过时阈值往上都允许，同一对出现多种结论时由优先级收敛。
func evolutionAllowedTypes(origin string, score float64) map[model.EvolutionAlertType]bool {
	if origin == EvolveOriginUpdateRelation {
		return map[model.EvolutionAlertType]bool{model.EvolutionAlertTypeOutdated: true}
	}
	if math.IsNaN(score) {
		return nil
	}
	allowed := make(map[model.EvolutionAlertType]bool, 3)
	if score >= EvolveDuplicateMinScore {
		allowed[model.EvolutionAlertTypeDuplicate] = true
	}
	if score >= EvolveConflictMinScore {
		allowed[model.EvolutionAlertTypeConflict] = true
	}
	if score >= EvolveOutdatedMinScore {
		allowed[model.EvolutionAlertTypeOutdated] = true
	}
	return allowed
}
