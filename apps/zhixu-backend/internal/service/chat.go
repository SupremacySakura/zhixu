package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
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
	RAG      bool          `json:"rag"`
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
	Texts    []string
	RagQuery *RagQuery
	// multiStep allows one chat turn to claim the same token for several model
	// calls (RAG rewrite + final answer). The token still dies with the request
	// context and is never reusable across turns.
	MultiStep bool
	claimed   bool
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
			if err := validateModelURL(p.BaseURL); err != nil {
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

func ValidateChat(input ChatInput) error {
	if len(input.Messages) == 0 || len(input.Messages) > 80 || len(input.Messages)%2 != 1 {
		return configInvalid("请提供交替排列、以用户消息结束的会话，最多 79 条消息")
	}
	total := 0
	for i, message := range input.Messages {
		expected := "user"
		if i%2 == 1 {
			expected = "assistant"
		}
		size := utf8.RuneCountInString(message.Content)
		if message.Role != expected || !utf8.ValidString(message.Content) || strings.TrimSpace(message.Content) == "" || size > 8000 {
			return configInvalid("消息角色或内容无效，每条最多 8000 字符")
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
	state, err := s.Store.Read(ctx, user)
	if err != nil {
		return nil, err
	}
	id := input.ModelID
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
			if err := validateModelURL(p.BaseURL); err != nil {
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

func (s *ChatService) ClaimCapability(token string) *ChatCapability {
	s.mu.Lock()
	defer s.mu.Unlock()
	cap := s.capabilities[token]
	if cap == nil || cap.Context.Err() != nil {
		return nil
	}
	// One-shot by default; multi-step tokens serve several model calls in one turn.
	if cap.claimed && !cap.MultiStep {
		return nil
	}
	cap.claimed = true
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

// Resolve once, validate every address, and dial the vetted IP directly to prevent DNS rebinding.
func (s *ChatService) ModelClient(ctx context.Context, base string) (*http.Client, func(), error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, nil, configInvalid("模型地址无效")
	}
	privateAllowed := false
	for _, entry := range strings.Split(s.AllowedPrivateEndpoints, ",") {
		if strings.TrimRight(strings.TrimSpace(entry), "/") == strings.TrimRight(base, "/") {
			privateAllowed = true
		}
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, u.Hostname())
	if err != nil || len(ips) == 0 {
		return nil, nil, errors.New("无法解析模型服务地址")
	}
	for _, addr := range ips {
		metadata := addr.IP.Equal(net.ParseIP("100.100.100.200")) || addr.IP.Equal(net.ParseIP("168.63.129.16"))
		if metadata || addr.IP.IsLinkLocalUnicast() || addr.IP.IsUnspecified() || addr.IP.IsMulticast() || (!privateAllowed && unsafeModelIP(addr.IP)) {
			return nil, nil, configInvalid("私网模型地址未获部署允许，或属于禁止访问的地址")
		}
	}
	if u.Scheme != "https" && !privateAllowed {
		return nil, nil, configInvalid("公网模型服务必须使用 HTTPS")
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		Proxy: nil, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 60 * time.Second,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
	}
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return client, transport.CloseIdleConnections, nil
}
