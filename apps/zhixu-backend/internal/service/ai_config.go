package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"
)

var (
	ErrAIConfigInvalid  = errors.New("模型配置参数无效")
	ErrAIConfigConflict = errors.New("模型配置冲突")
	ErrAIConfigNotFound = errors.New("模型配置不存在")
	providerIDPattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
)

type AIConfigStore interface {
	Read(context.Context, uint) (*repository.AIConfigState, error)
	Mutate(context.Context, uint, func(*repository.AIConfigState) error) (*repository.AIConfigState, error)
}

type AIConfigService struct {
	Store       AIConfigStore
	Credentials *ModelCredentials
}

type ModelInput struct {
	ModelID   string `json:"model_id"`
	Kind      string `json:"kind"`
	Dimension uint   `json:"dimension"`
}

type ProviderInput struct {
	ProviderID      string       `json:"provider_id"`
	Name            string       `json:"name"`
	CatalogID       string       `json:"catalog_id"`
	BaseURL         string       `json:"base_url"`
	Protocol        string       `json:"protocol"`
	AuthType        string       `json:"auth_type"`
	APIKey          *string      `json:"api_key"`
	ClearAPIKey     bool         `json:"clear_api_key"`
	Models          []ModelInput `json:"models"`
	ExpectedVersion uint64       `json:"expected_version"`
	ConfirmReindex  bool         `json:"confirm_reindex"`
}

type DefaultsInput struct {
	ChatModelID      uint     `json:"chat_model_id"`
	EmbeddingModelID uint     `json:"embedding_model_id"`
	// Nil RagTopK/RagMinScore keeps the stored retrieval settings, so clients
	// predating them can still save model defaults without resetting retrieval.
	RagTopK         *int     `json:"rag_top_k"`
	RagMinScore     *float64 `json:"rag_min_score"`
	ExpectedVersion uint64   `json:"expected_version"`
	ConfirmReindex  bool     `json:"confirm_reindex"`
}

type ProviderView struct {
	model.ModelProvider
	CredentialConfigured bool                  `json:"credential_configured"`
	Models               []model.ProviderModel `json:"models"`
}

type AIConfigView struct {
	Providers []ProviderView       `json:"providers"`
	Settings  model.UserAISettings `json:"settings"`
}

type ProviderPreset struct {
	ProviderID string `json:"provider_id"`
	Name       string `json:"name"`
	BaseURL    string `json:"base_url"`
	Protocol   string `json:"protocol"`
	AuthType   string `json:"auth_type"`
}

func AIProviderCatalog() []ProviderPreset {
	return []ProviderPreset{
		{"deepseek", "DeepSeek", "https://api.deepseek.com", "chat", "key"},
		{"openai", "OpenAI", "https://api.openai.com/v1", "chat", "key"},
		{"siliconflow", "硅基流动", "https://api.siliconflow.cn/v1", "chat", "key"},
	}
}

func aiView(state *repository.AIConfigState) *AIConfigView {
	view := &AIConfigView{Providers: []ProviderView{}, Settings: state.Settings}
	for _, p := range state.Providers {
		entry := ProviderView{ModelProvider: p, CredentialConfigured: p.Ciphertext != "", Models: []model.ProviderModel{}}
		// Clear secrets even in the in-memory public response, not just through JSON tags.
		entry.Ciphertext = ""
		for _, m := range state.Models {
			if m.ProviderID == p.ID {
				entry.Models = append(entry.Models, m)
			}
		}
		view.Providers = append(view.Providers, entry)
	}
	return view
}

func (s *AIConfigService) Get(ctx context.Context, user uint) (*AIConfigView, error) {
	state, err := s.Store.Read(ctx, user)
	if err != nil {
		return nil, err
	}
	return aiView(state), nil
}

func configInvalid(message string) error  { return fmt.Errorf("%w: %s", ErrAIConfigInvalid, message) }
func configConflict(message string) error { return fmt.Errorf("%w: %s", ErrAIConfigConflict, message) }

// No endpoint is contacted here. DNS/address validation must also run at future call time.
func validateModelURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") ||
		u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || len(raw) > 2048 {
		return configInvalid("请填写不含凭据、查询参数和片段的 HTTP(S) 基础地址")
	}
	if strings.HasSuffix(u.Path, "/embeddings") || strings.HasSuffix(u.Path, "/chat/completions") ||
		strings.HasSuffix(u.Path, "/responses") || strings.HasSuffix(u.Path, "/messages") {
		return configInvalid("请填写基础地址，不包含具体模型接口路径")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && (ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) {
		return configInvalid("不允许使用未指定、多播或链路本地地址")
	}
	if port := u.Port(); port != "" {
		var n int
		if _, err := fmt.Sscanf(port, "%d", &n); err != nil || n < 1 || n > 65535 {
			return configInvalid("端口无效")
		}
	}
	return nil
}

func validateProvider(input *ProviderInput) error {
	input.ProviderID = strings.TrimSpace(input.ProviderID)
	input.Name = strings.TrimSpace(input.Name)
	input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	if !providerIDPattern.MatchString(input.ProviderID) {
		return configInvalid("Provider ID 格式无效")
	}
	if !utf8.ValidString(input.Name) || utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 60 {
		return configInvalid("显示名称长度须为 1–60 字符")
	}
	if input.Protocol != "chat" && input.Protocol != "responses" {
		return configInvalid("首版仅支持 OpenAI Chat Completions 或 Responses 协议")
	}
	if input.AuthType != "key" && input.AuthType != "none" {
		return configInvalid("认证方式必须是 key 或 none")
	}
	if input.ClearAPIKey && input.APIKey != nil && *input.APIKey != "" {
		return configInvalid("不能同时替换和清除密钥")
	}
	if input.AuthType == "none" && input.APIKey != nil && *input.APIKey != "" {
		return configInvalid("无认证连接不能提交密钥")
	}
	if input.APIKey != nil && (len(*input.APIKey) > 8192 || strings.ContainsAny(*input.APIKey, "\r\n\x00")) {
		return configInvalid("密钥格式无效")
	}
	if err := validateModelURL(input.BaseURL); err != nil {
		return err
	}
	if input.CatalogID != "" {
		found := false
		for _, preset := range AIProviderCatalog() {
			if preset.ProviderID == input.CatalogID {
				found = input.ProviderID == preset.ProviderID && input.BaseURL == preset.BaseURL && input.Protocol == preset.Protocol && input.AuthType == preset.AuthType
			}
		}
		if !found {
			return configInvalid("预设连接与供应商目录不匹配；其他端点请使用自定义 API")
		}
	}
	if len(input.Models) == 0 || len(input.Models) > 100 {
		return configInvalid("每个供应商须包含 1–100 个模型")
	}
	seen := map[string]bool{}
	for i := range input.Models {
		m := &input.Models[i]
		m.ModelID = strings.TrimSpace(m.ModelID)
		// MySQL's default collation is case-insensitive; validate on the same boundary.
		key := strings.ToLower(m.ModelID)
		if !utf8.ValidString(m.ModelID) || m.ModelID == "" || utf8.RuneCountInString(m.ModelID) > 200 || strings.ContainsAny(m.ModelID, "\r\n\x00") || seen[key] {
			return configInvalid("模型 ID 无效或重复")
		}
		seen[key] = true
		if m.Kind != "chat" && m.Kind != "embedding" {
			return configInvalid("模型用途必须是 chat 或 embedding")
		}
		if m.Kind == "embedding" && (m.Dimension == 0 || m.Dimension > 65536) {
			return configInvalid("Embedding 维度须为 1–65536")
		}
		if m.Kind == "chat" && m.Dimension != 0 {
			return configInvalid("对话模型不应填写向量维度")
		}
		if input.CatalogID == "deepseek" && m.Kind == "embedding" {
			return configInvalid("DeepSeek 预设不支持 Embedding")
		}
	}
	return nil
}

func embeddingFingerprint(state *repository.AIConfigState) string {
	id := state.Settings.EmbeddingModelID
	if id == 0 {
		return ""
	}
	for _, m := range state.Models {
		if m.ID != id {
			continue
		}
		for _, p := range state.Providers {
			if p.ID == m.ProviderID {
				return fmt.Sprintf("%d|%s|%s|%d", p.ID, p.BaseURL, m.ModelID, m.Dimension)
			}
		}
	}
	return ""
}

func checkDefaults(state *repository.AIConfigState, requireCredentials bool) error {
	for _, selected := range []struct {
		id   uint
		kind string
	}{{state.Settings.ChatModelID, "chat"}, {state.Settings.EmbeddingModelID, "embedding"}} {
		if selected.id == 0 {
			continue
		}
		found := false
		for _, m := range state.Models {
			if m.ID != selected.id {
				continue
			}
			if m.Kind != selected.kind {
				return configConflict("默认模型用途不匹配，请先切换默认模型")
			}
			for _, p := range state.Providers {
				if p.ID != m.ProviderID {
					continue
				}
				found = true
				if requireCredentials && p.AuthType == "key" && p.Ciphertext == "" {
					return configInvalid("默认模型供应商尚未配置密钥")
				}
			}
		}
		if !found {
			return configConflict("默认模型仍在使用此供应商或模型，请先切换默认模型")
		}
	}
	return nil
}

func applyEmbeddingChange(state *repository.AIConfigState, previous string, confirmed bool) error {
	next := embeddingFingerprint(state)
	if previous == next {
		return nil
	}
	if previous != "" && !confirmed {
		return configConflict("更换 Embedding 配置需要重建向量，请提交 confirm_reindex=true")
	}
	state.Settings.EmbeddingVersion++
	if previous != "" {
		state.Settings.NeedsReindex = true
	}
	return nil
}

func (s *AIConfigService) SaveProvider(ctx context.Context, user, id uint, input ProviderInput) (*AIConfigView, error) {
	if err := validateProvider(&input); err != nil {
		return nil, err
	}
	ciphertext := ""
	if input.APIKey != nil && *input.APIKey != "" {
		var err error
		ciphertext, err = s.Credentials.Encrypt(user, input.ProviderID, *input.APIKey)
		if err != nil {
			return nil, err
		}
	}
	state, err := s.Store.Mutate(ctx, user, func(state *repository.AIConfigState) error {
		if state.Settings.ConfigVersion != input.ExpectedVersion {
			return configConflict("配置已变更，请刷新后重试")
		}
		previous := embeddingFingerprint(state)
		index := -1
		for i, p := range state.Providers {
			if id != 0 && p.ID == id {
				index = i
			}
			if p.ProviderID == input.ProviderID && p.ID != id {
				return configConflict("Provider ID 已存在")
			}
		}
		if id != 0 && index < 0 {
			return ErrAIConfigNotFound
		}
		p := model.ModelProvider{ID: id, UserID: user, ProviderID: input.ProviderID, CatalogID: input.CatalogID}
		if index >= 0 {
			p = state.Providers[index]
			if p.ProviderID != input.ProviderID || p.CatalogID != input.CatalogID {
				return configInvalid("Provider ID 与目录归属保存后不可修改")
			}
		} else if len(state.Providers) >= 32 {
			return configInvalid("最多配置 32 个供应商")
		}
		p.Name, p.BaseURL, p.Protocol, p.AuthType = input.Name, input.BaseURL, input.Protocol, input.AuthType
		if ciphertext != "" {
			p.Ciphertext = ciphertext
		}
		if input.ClearAPIKey || input.AuthType == "none" {
			p.Ciphertext = ""
		}
		if index < 0 {
			state.Providers = append(state.Providers, p)
		} else {
			state.Providers[index] = p
		}
		old := map[string]model.ProviderModel{}
		kept := []model.ProviderModel{}
		for _, m := range state.Models {
			if id != 0 && m.ProviderID == id {
				old[m.ModelID] = m
			} else {
				kept = append(kept, m)
			}
		}
		for _, m := range input.Models {
			entry := old[m.ModelID]
			entry.UserID, entry.ProviderID, entry.ModelID, entry.Kind, entry.Dimension = user, id, m.ModelID, m.Kind, m.Dimension
			kept = append(kept, entry)
		}
		state.Models = kept
		if err := checkDefaults(state, false); err != nil {
			return err
		}
		return applyEmbeddingChange(state, previous, input.ConfirmReindex)
	})
	if err != nil {
		return nil, err
	}
	return aiView(state), nil
}

func (s *AIConfigService) DeleteProvider(ctx context.Context, user, id uint, version uint64) (*AIConfigView, error) {
	state, err := s.Store.Mutate(ctx, user, func(state *repository.AIConfigState) error {
		if state.Settings.ConfigVersion != version {
			return configConflict("配置已变更，请刷新后重试")
		}
		found := false
		providers := []model.ModelProvider{}
		for _, p := range state.Providers {
			if p.ID == id {
				found = true
			} else {
				providers = append(providers, p)
			}
		}
		if !found {
			return ErrAIConfigNotFound
		}
		models := []model.ProviderModel{}
		for _, m := range state.Models {
			if m.ProviderID != id {
				models = append(models, m)
			}
		}
		state.Providers, state.Models = providers, models
		return checkDefaults(state, false)
	})
	if err != nil {
		return nil, err
	}
	return aiView(state), nil
}

func (s *AIConfigService) SaveDefaults(ctx context.Context, user uint, input DefaultsInput) (*AIConfigView, error) {
	if input.RagTopK != nil && (*input.RagTopK < 1 || *input.RagTopK > RagMaxTopK) {
		return nil, configInvalid(fmt.Sprintf("检索条数需在 1–%d 之间", RagMaxTopK))
	}
	if input.RagMinScore != nil && (*input.RagMinScore < RagMinScoreFloor || *input.RagMinScore > 1) {
		return nil, configInvalid(fmt.Sprintf("相似度阈值需在 %g–1 之间", RagMinScoreFloor))
	}
	state, err := s.Store.Mutate(ctx, user, func(state *repository.AIConfigState) error {
		if input.ExpectedVersion != state.Settings.ConfigVersion {
			return configConflict("配置已变更，请刷新后重试")
		}
		previous := embeddingFingerprint(state)
		state.Settings.ChatModelID, state.Settings.EmbeddingModelID = input.ChatModelID, input.EmbeddingModelID
		if input.RagTopK != nil {
			state.Settings.RagTopK = *input.RagTopK
		}
		if input.RagMinScore != nil {
			state.Settings.RagMinScore = *input.RagMinScore
		}
		if err := checkDefaults(state, true); err != nil {
			return err
		}
		return applyEmbeddingChange(state, previous, input.ConfirmReindex)
	})
	if err != nil {
		return nil, err
	}
	return aiView(state), nil
}
