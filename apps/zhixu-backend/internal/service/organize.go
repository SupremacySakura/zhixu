package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// OrganizeContentMaxRunes 单次整理送给模型的正文上限（字符），
	// 与 AI 侧 zod schema 的 content 上限 12000 保持一致：超长会在契约层被 400 拒绝，整任务白跑。
	OrganizeContentMaxRunes = 12_000
	// OrganizeExistingTagMax 与 AI 侧 existingTags 的上限一致。
	OrganizeExistingTagMax = 50
	// OrganizeSummaryMaxRunes 采纳后的摘要上限（字符）。
	OrganizeSummaryMaxRunes = 200
	// OrganizeKeywordMax 采纳后的关键词数量上限。
	OrganizeKeywordMax = 10
	// OrganizeKeywordMaxRunes 采纳后的单个关键词长度上限（字符）。
	OrganizeKeywordMaxRunes = 20
)

// AI 整理调用的错误分类：调用方据此决定是重试还是直接判任务失败。
var (
	// ErrOrganizeInvalidOutput 表示模型输出不符合结构化契约（AI 侧 422 schema_invalid）。
	// 同一模型重试通常得到同样的错误输出，因此按不可重试处理。
	ErrOrganizeInvalidOutput = errors.New("AI 输出格式不符合要求")
	// ErrOrganizeRejected 表示 AI 服务拒绝了请求本身（400/403），属于契约或配置错误，重试无意义。
	ErrOrganizeRejected = errors.New("AI 整理请求被拒绝")
)

// OrganizeDraft 是 AI 整理产出的草稿。它先落在 ai_tasks.payload，用户确认后才写入 knowledge_items 的正式字段。
type OrganizeDraft struct {
	Summary       string   `json:"summary"`
	Keywords      []string `json:"keywords"`
	SuggestedTags []string `json:"suggested_tags"`
	Concepts      []string `json:"concepts"`
}

// OrganizePayload 是落在 ai_tasks.payload 的整理产物：模型草稿 + 生成时的正文版本 + 采纳时间。
// 过期判断用正文版本而不是时间戳：采纳动作本身会刷新 knowledge_items.updated_at。
type OrganizePayload struct {
	OrganizeDraft
	BaseVersion uint       `json:"base_version"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
}

// OrganizeTagView 是推荐标签的接口视图：tag_id 非零表示用户已有同名标签，前端可直接勾选。
type OrganizeTagView struct {
	Name     string `json:"name"`
	TagID    uint   `json:"tag_id"`
	Existing bool   `json:"existing"`
}

// OrganizeDraftView 是详情页预览面板的数据：草稿内容 + 标签解析结果 + 采纳/过期状态。
type OrganizeDraftView struct {
	Summary       string            `json:"summary"`
	Keywords      []string          `json:"keywords"`
	SuggestedTags []OrganizeTagView `json:"suggested_tags"`
	Concepts      []string          `json:"concepts"`
	Accepted      bool              `json:"accepted"`
	AcceptedAt    *time.Time        `json:"accepted_at,omitempty"`
	// Stale 表示草稿生成后正文又被编辑过，采纳前应提示用户重新生成。
	Stale       bool `json:"stale"`
	BaseVersion uint `json:"base_version"`
}

// OrganizeClient 调用 AI 服务的 /organize。模型配置随请求下发，AI 服务自身不保管凭据。
// Secret 是内部共享的 bearer：/organize 会驱动出站模型调用，不能对未认证调用者开放。
type OrganizeClient struct {
	AIURL  string
	Secret string
}

// Organize 请求一次结构化整理。content 在这里再次截断，避免调用方漏截；
// 成功返回已清洗的草稿（去空、去重、按契约截断），失败按错误分类返回。
func (c *OrganizeClient) Organize(ctx context.Context, snapshot *ChatCapability, taskID, knowledgeID uint, title, content string, existingTags []string) (*OrganizeDraft, error) {
	if c == nil || snapshot == nil {
		return nil, errors.New("AI 整理未配置")
	}
	if strings.TrimSpace(c.Secret) == "" {
		return nil, errors.New("INTERNAL_API_SECRET 未配置，无法调用 AI 服务")
	}
	if content = strings.TrimSpace(content); content == "" {
		return nil, fmt.Errorf("%w: 正文为空，无法整理", ErrOrganizeRejected)
	}
	tags := make([]string, 0, len(existingTags))
	for _, tag := range existingTags {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, tag)
		}
		if len(tags) == OrganizeExistingTagMax {
			break
		}
	}
	payload, err := json.Marshal(map[string]any{
		"version": 1,
		"model": map[string]any{
			"protocol": snapshot.Provider.Protocol, "model_id": snapshot.Model.ModelID,
			"base_url": snapshot.Provider.BaseURL, "api_key": snapshot.Key,
		},
		"taskId":       taskID,
		"knowledgeId":  knowledgeID,
		"title":        truncateRunes(strings.TrimSpace(title), 200),
		"content":      truncateRunes(content, OrganizeContentMaxRunes),
		"existingTags": tags,
	})
	if err != nil {
		return nil, errors.New("AI 整理请求构造失败")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.AIURL, "/")+"/organize", bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("AI 服务地址配置无效")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.Secret)
	// 不自动重试：重试会重复一次付费模型调用，重试策略由任务状态机统一负责。
	client := &http.Client{
		Timeout:       100 * time.Second,
		Transport:     &http.Transport{Proxy: nil, ResponseHeaderTimeout: 95 * time.Second},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	response, err := client.Do(request)
	if err != nil {
		return nil, errors.New("AI 服务未启动或响应超时，请检查 Mastra 服务")
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == http.StatusOK:
	case response.StatusCode == http.StatusUnprocessableEntity:
		return nil, ErrOrganizeInvalidOutput
	case response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("%w: 请检查 AI 服务与模型配置（HTTP %d）", ErrOrganizeRejected, response.StatusCode)
	case response.StatusCode == http.StatusGatewayTimeout:
		return nil, errors.New("AI 整理调用超时，请稍后重试")
	default:
		return nil, fmt.Errorf("AI 整理服务暂不可用（HTTP %d）", response.StatusCode)
	}
	var result struct {
		Draft OrganizeDraft `json:"draft"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return nil, errors.New("AI 服务返回了无效的整理结果")
	}
	draft, err := sanitizeDraft(result.Draft)
	if err != nil {
		return nil, err
	}
	return draft, nil
}

// sanitizeDraft 清洗模型输出：去掉空白项与重复项，并按落库维度截断。
// 超长摘要说明模型没遵守指令，此时整任务判失败比静默截断更诚实。
func sanitizeDraft(draft OrganizeDraft) (*OrganizeDraft, error) {
	summary := strings.TrimSpace(draft.Summary)
	if summary == "" {
		return nil, ErrOrganizeInvalidOutput
	}
	if utf8.RuneCountInString(summary) > OrganizeSummaryMaxRunes {
		return nil, fmt.Errorf("%w: 摘要超过 %d 字符", ErrOrganizeInvalidOutput, OrganizeSummaryMaxRunes)
	}
	keywords := cleanItems(draft.Keywords, 20, OrganizeKeywordMax)
	if len(keywords) == 0 {
		return nil, fmt.Errorf("%w: 缺少关键词", ErrOrganizeInvalidOutput)
	}
	clean := &OrganizeDraft{
		Summary:       summary,
		Keywords:      keywords,
		SuggestedTags: cleanItems(draft.SuggestedTags, TagNameMaxRunes, 6),
		Concepts:      cleanItems(draft.Concepts, 30, 6),
	}
	return clean, nil
}

// cleanItems 逐项 trim、丢弃空项与重复项，并按 maxRunes/maxItems 截断。
func cleanItems(items []string, maxRunes, maxItems int) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || utf8.RuneCountInString(item) > maxRunes {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
		if len(out) == maxItems {
			break
		}
	}
	return out
}

// ParseKeywords 把 knowledge_items.keywords 列解析为接口视图；空串或非法 JSON 都退化为空数组。
func ParseKeywords(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return []string{}
	}
	return cleanItems(list, 20, OrganizeKeywordMax)
}

// EncodeKeywords 把关键词列表编码为落库字符串；空列表存空串而不是 "null"。
func EncodeKeywords(keywords []string) string {
	clean := cleanItems(keywords, 20, OrganizeKeywordMax)
	if len(clean) == 0 {
		return ""
	}
	encoded, err := json.Marshal(clean)
	if err != nil {
		return ""
	}
	return string(encoded)
}
