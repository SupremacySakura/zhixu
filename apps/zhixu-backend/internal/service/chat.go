package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/url"
	"strings"
	"sync"
	"unicode/utf8"

	"zhixu-backend/internal/model"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type ChatInput struct {
	ModelID  uint          `json:"model_id"`
	Messages []ChatMessage `json:"messages"`
	// ConversationID, when set, continues an existing Mastra thread; when empty the
	// backend starts a new thread and returns its id in the meta event. Title seeds the
	// new thread (first user message, truncated) so the AI service never spends a model
	// call on title generation.
	ConversationID string `json:"conversation_id"`
	Title          string `json:"title"`
}
type ChatCapability struct {
	Model    model.ProviderModel
	Provider model.ModelProvider
	Key      string
	Context  context.Context
	RagQuery *RagQuery
}

func (s *ChatService) ResolveEmbedding(ctx context.Context, user uint) (*ChatCapability, uint64, uint64, error) {
	state, err := s.Store.Read(ctx, user)
	if err != nil {
		return nil, 0, 0, err
	}
	for _, m := range state.Models {
		if m.ID != state.Settings.EmbeddingModelID || m.Kind != "embedding" || m.Dimension == 0 {
			continue
		}
		for _, p := range state.Providers {
			if p.ID != m.ProviderID {
				continue
			}
			if err := s.validateRuntimeModelURL(p.BaseURL); err != nil {
				return nil, 0, 0, err
			}
			cap := &ChatCapability{Model: m, Provider: p, Context: ctx}
			if p.AuthType == "key" {
				if p.Ciphertext == "" {
					return nil, 0, 0, configInvalid("供应商未配置 API Key")
				}
				cap.Key, err = s.Credentials.Decrypt(user, p.ProviderID, p.Ciphertext)
				if err != nil {
					return nil, 0, 0, ErrModelKeyUnavailable
				}
			}
			return cap, state.Settings.EmbeddingVersion, state.Settings.ConfigVersion, nil
		}
	}
	return nil, 0, 0, configInvalid("未配置可用的 Embedding 模型，请前往模型设置")
}

type ChatService struct {
	Store                   AIConfigStore
	Credentials             *ModelCredentials
	AllowedPrivateEndpoints string
	mu                      sync.Mutex
	capabilities            map[string]*ChatCapability
}

// NormalizeChatHistory repairs the strict user/assistant alternation provider APIs
// expect when a thread contains turns that never produced visible text. Mastra persists
// every step of a multi-step turn, so an interrupted, failed, or aborted answer leaves an
// assistant message whose only parts are reasoning and tool invocations. Replaying that
// empty turn verbatim makes the provider reject the whole request ("消息角色或内容无效"),
// so such turns are dropped and adjacent user messages merged.
func NormalizeChatHistory(messages []ChatMessage) []ChatMessage {
	out := make([]ChatMessage, 0, len(messages))
	for _, message := range messages {
		if message.Role == "assistant" && strings.TrimSpace(message.Content) == "" {
			continue
		}
		if message.Role == "user" && len(out) > 0 && out[len(out)-1].Role == "user" {
			previous := strings.TrimSpace(out[len(out)-1].Content)
			if current := strings.TrimSpace(message.Content); current != "" {
				if previous != "" {
					previous += "\n\n" + current
				}
				out[len(out)-1].Content = previous
			}
			continue
		}
		out = append(out, message)
	}
	return out
}

// ChatMessageMaxRunes bounds the message being sent in this turn. Only the final
// user turn is capped at 2000, keeping prompts short enough for grounded retrieval;
// historical messages keep the larger legacy cap so opened conversations stay usable.
const ChatMessageMaxRunes = 2000

// ChatHistoryMaxRunes bounds every historical message (and every assistant reply).
const ChatHistoryMaxRunes = 8000

func ValidateChat(input ChatInput) error {
	if len(input.Messages) == 0 || len(input.Messages) > 80 || len(input.Messages)%2 != 1 {
		return configInvalid("请提供交替排列、以用户消息结束的会话，最多 79 条消息")
	}
	total := 0
	last := len(input.Messages) - 1
	for i, message := range input.Messages {
		expected := "user"
		if i%2 == 1 {
			expected = "assistant"
		}
		limit := ChatHistoryMaxRunes
		if i == last {
			limit = ChatMessageMaxRunes
		}
		size := utf8.RuneCountInString(message.Content)
		if message.Role != expected || !utf8.ValidString(message.Content) || strings.TrimSpace(message.Content) == "" || size > limit {
			return configInvalid("消息角色或内容无效：本次提问最多 2000 字符，历史消息与回答最多 8000 字符")
		}
		total += size
	}
	if total > 60000 {
		return configInvalid("会话上下文超过 60000 字符，请开始新对话")
	}
	return nil
}

func (s *ChatService) Resolve(ctx context.Context, user uint, input ChatInput) (*ChatCapability, error) {
	if err := ValidateChat(input); err != nil {
		return nil, err
	}
	return s.resolveModel(ctx, user, input.ModelID)
}

// ResolveChatModel 解析默认对话模型能力，供后台 AI 任务（整理、关联）复用：
// 这些任务不是会话，不适用 ValidateChat 的消息交替与长度约束。
func (s *ChatService) ResolveChatModel(ctx context.Context, user uint) (*ChatCapability, error) {
	return s.resolveModel(ctx, user, 0)
}

func (s *ChatService) resolveModel(ctx context.Context, user uint, modelID uint) (*ChatCapability, error) {
	state, err := s.Store.Read(ctx, user)
	if err != nil {
		return nil, err
	}
	id := modelID
	if id == 0 {
		id = state.Settings.ChatModelID
	}
	for _, m := range state.Models {
		if m.ID != id || m.Kind != "chat" {
			continue
		}
		for _, p := range state.Providers {
			if p.ID != m.ProviderID {
				continue
			}
			if err := s.validateRuntimeModelURL(p.BaseURL); err != nil {
				return nil, err
			}
			cap := &ChatCapability{Model: m, Provider: p, Context: ctx}
			if p.AuthType == "key" {
				if p.Ciphertext == "" {
					return nil, configInvalid("供应商未配置 API Key，请前往设置")
				}
				cap.Key, err = s.Credentials.Decrypt(user, p.ProviderID, p.Ciphertext)
				if err != nil {
					return nil, ErrModelKeyUnavailable
				}
			}
			return cap, nil
		}
	}
	return nil, configInvalid("未配置可用的对话模型，请前往模型设置")
}

func (s *ChatService) Register(cap *ChatCapability) (string, func(), error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", nil, err
	}
	token := hex.EncodeToString(bytes)
	s.mu.Lock()
	if s.capabilities == nil {
		s.capabilities = map[string]*ChatCapability{}
	}
	s.capabilities[token] = cap
	s.mu.Unlock()
	return token, func() { s.mu.Lock(); delete(s.capabilities, token); s.mu.Unlock() }, nil
}
func (s *ChatService) Capability(token string) *ChatCapability {
	s.mu.Lock()
	defer s.mu.Unlock()
	cap := s.capabilities[token]
	if cap != nil && cap.Context.Err() != nil {
		return nil
	}
	return cap
}

// ClaimRagCapability hands a retrieval token to the RAG search gateway. Unlike model
// capabilities it may be claimed several times within one chat turn, because the agent
// can search more than once; it still expires with the request context.
func (s *ChatService) ClaimRagCapability(token string) *ChatCapability {
	s.mu.Lock()
	defer s.mu.Unlock()
	cap := s.capabilities[token]
	if cap == nil || cap.RagQuery == nil || cap.Context.Err() != nil {
		return nil
	}
	return cap
}

func unsafeModelIP(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() || !ip.IsGlobalUnicast() ||
		ip.Equal(net.ParseIP("100.100.100.200")) || ip.Equal(net.ParseIP("168.63.129.16")) ||
		(&net.IPNet{IP: net.ParseIP("100.64.0.0"), Mask: net.CIDRMask(10, 32)}).Contains(ip)
}

// privateEndpointAllowed reports whether the deployment explicitly trusts this exact
// provider base URL (comma-separated, exact match after trimming the trailing slash).
func (s *ChatService) privateEndpointAllowed(base string) bool {
	for _, entry := range strings.Split(s.AllowedPrivateEndpoints, ",") {
		if strings.TrimRight(strings.TrimSpace(entry), "/") == strings.TrimRight(base, "/") {
			return true
		}
	}
	return false
}

// validateRuntimeModelURL applies the deployment address policy when a provider is about
// to be used. Saving stays permissive so configuration can be staged before the allowlist
// is set; borrowing a public API over plain HTTP or pointing at loopback/metadata ranges
// requires an exact allowlist entry. The AI service enforces the same policy again at
// request time (apps/zhixu-ai/src/lib/egress-guard.ts): it rejects unsafe literal
// addresses (including alternative IPv4 spellings that Go's parser does not recognize),
// hostnames that resolve to private/reserved answers, and every non-exempt redirect hop
// that is not HTTPS; only the exact backend origin and EGRESS_ALLOWED_HOSTS are exempt.
// This check is a first policy gate rather than the only one.
func (s *ChatService) validateRuntimeModelURL(raw string) error {
	if err := validateModelURL(raw); err != nil {
		return err
	}
	u, err := url.Parse(raw)
	if err != nil {
		return configInvalid("模型地址无效")
	}
	allowed := s.privateEndpointAllowed(raw)
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		if !allowed {
			return configInvalid("私网模型地址未获部署允许，请配置精确的允许地址")
		}
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if unsafeModelIP(ip) && !allowed {
			return configInvalid("私网模型地址未获部署允许，请配置精确的允许地址")
		}
		return nil
	}
	if u.Scheme != "https" && !allowed {
		return configInvalid("公网模型服务必须使用 HTTPS")
	}
	return nil
}
