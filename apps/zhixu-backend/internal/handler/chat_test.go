package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChatSendAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/chat", (&ChatHandler{}).Send)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/chat", strings.NewReader("{}")))
	require.Equal(t, 401, w.Code)
}

func TestChatSendRejectsMissingInternalSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/chat", func(c *gin.Context) {
		c.Set("user_id", uint(7))
		(&ChatHandler{}).Send(c)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/chat", strings.NewReader("{}")))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestChatSendCarriesToolCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var path string
	var payload struct {
		Version int `json:"version"`
		Model   struct {
			Protocol string `json:"protocol"`
			ModelID  string `json:"model_id"`
			BaseURL  string `json:"base_url"`
			APIKey   string `json:"api_key"`
		} `json:"model"`
		Tools struct {
			Capability string `json:"capability"`
			SearchURL  string `json:"search_url"`
		} `json:"tools"`
	}
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		require.Equal(t, "Bearer internal-secret", r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte("{\"type\":\"done\"}\n"))
	}))
	defer ai.Close()
	s := &service.ChatService{Store: ragChatStore{}, AllowedPrivateEndpoints: "http://127.0.0.1:11434/v1"}
	h := &ChatHandler{Service: s, AIURL: ai.URL, InternalAPISecret: "internal-secret", InternalBackendURL: "http://127.0.0.1:9000", Rag: &service.RagSearcher{}}
	r := gin.New()
	r.POST("/chat", func(c *gin.Context) { c.Set("user_id", uint(7)); h.Send(c) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/chat",
		strings.NewReader(`{"messages":[{"role":"user","content":"回滚步骤"}]}`)))
	require.Equal(t, 200, w.Code)
	require.Equal(t, "/chat", path)
	require.Equal(t, 1, payload.Version)
	require.Equal(t, "chat", payload.Model.Protocol)
	require.Equal(t, "chat-model", payload.Model.ModelID)
	require.Equal(t, "http://127.0.0.1:11434/v1", payload.Model.BaseURL)
	require.Len(t, payload.Tools.Capability, 64)
	require.Equal(t, "http://127.0.0.1:9000/internal/tools/v1/search", payload.Tools.SearchURL)
	require.Nil(t, s.Capability(payload.Tools.Capability))
}

type ragChatStore struct{ handlerAIStore }

func (ragChatStore) Read(_ context.Context, user uint) (*repository.AIConfigState, error) {
	return &repository.AIConfigState{
		Providers: []model.ModelProvider{{ID: 1, UserID: user, BaseURL: "http://127.0.0.1:11434/v1", AuthType: "none", Protocol: "chat"}},
		Models: []model.ProviderModel{
			{ID: 2, ProviderID: 1, UserID: user, Kind: "chat", ModelID: "chat-model"},
			{ID: 4, ProviderID: 1, UserID: user, Kind: "embedding", ModelID: "embeddinggemma", Dimension: 768},
		},
		Settings: model.UserAISettings{ChatModelID: 2, EmbeddingModelID: 4, RagTopK: 12, RagMinScore: 0.6},
	}, nil
}

type chatHandlerStore struct{ handlerAIStore }

func (chatHandlerStore) Read(_ context.Context, user uint) (*repository.AIConfigState, error) {
	return &repository.AIConfigState{
		Providers: []model.ModelProvider{{ID: 1, UserID: user, Name: "本地模型", BaseURL: "http://127.0.0.1:11434/v1", AuthType: "none", Protocol: "chat"}},
		Models:    []model.ProviderModel{{ID: 2, ProviderID: 1, UserID: user, Kind: "chat", ModelID: "test-model"}},
		Settings:  model.UserAISettings{ChatModelID: 2},
	}, nil
}

func TestChatSendStreamsViaMastraRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var capability string
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/chat", r.URL.Path)
		require.Equal(t, "Bearer internal-secret", r.Header.Get("Authorization"))
		var payload struct {
			Version int `json:"version"`
			Model   struct {
				Protocol string `json:"protocol"`
				ModelID  string `json:"model_id"`
				BaseURL  string `json:"base_url"`
			} `json:"model"`
			Tools struct {
				Capability string `json:"capability"`
				SearchURL  string `json:"search_url"`
			} `json:"tools"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, 1, payload.Version)
		require.Equal(t, "chat", payload.Model.Protocol)
		require.Equal(t, "test-model", payload.Model.ModelID)
		require.Equal(t, "http://127.0.0.1:11434/v1", payload.Model.BaseURL)
		require.Equal(t, "http://127.0.0.1:9000/internal/tools/v1/search", payload.Tools.SearchURL)
		capability = payload.Tools.Capability
		require.Len(t, capability, 64)
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte("{\"type\":\"delta\",\"text\":\"你好\"}\n{\"type\":\"done\"}\n"))
	}))
	defer ai.Close()
	s := &service.ChatService{Store: chatHandlerStore{}, AllowedPrivateEndpoints: "http://127.0.0.1:11434/v1"}
	h := &ChatHandler{Service: s, AIURL: ai.URL, InternalAPISecret: "internal-secret", InternalBackendURL: "http://127.0.0.1:9000", Rag: &service.RagSearcher{}}
	r := gin.New()
	r.POST("/chat", func(c *gin.Context) { c.Set("user_id", uint(7)); h.Send(c) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/chat", strings.NewReader(`{"messages":[{"role":"user","content":"你好"}]}`)))
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"type":"meta"`)
	require.Contains(t, w.Body.String(), `"type":"done"`)
	require.NotContains(t, w.Body.String(), capability)
	require.Nil(t, s.Capability(capability))
}

func TestChatSendDropsInterruptedTurnsBeforeValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/chat", r.URL.Path)
		var payload struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{{Role: "user", Content: "第一个问题\n\n第二个问题"}}, payload.Messages)
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte("{\"type\":\"done\"}\n"))
	}))
	defer ai.Close()
	s := &service.ChatService{Store: chatHandlerStore{}, AllowedPrivateEndpoints: "http://127.0.0.1:11434/v1"}
	h := &ChatHandler{Service: s, AIURL: ai.URL, InternalAPISecret: "internal-secret", InternalBackendURL: "http://127.0.0.1:9000", Rag: &service.RagSearcher{}}
	r := gin.New()
	r.POST("/chat", func(c *gin.Context) { c.Set("user_id", uint(7)); h.Send(c) })
	w := httptest.NewRecorder()
	body := `{"messages":[{"role":"user","content":"第一个问题"},{"role":"assistant","content":"\n "},{"role":"user","content":"第二个问题"}]}`
	r.ServeHTTP(w, httptest.NewRequest("POST", "/chat", strings.NewReader(body)))
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"type":"done"`)
}
