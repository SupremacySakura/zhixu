package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"zhixu-backend/internal/model"

	"github.com/stretchr/testify/require"
)

const testInternalSecret = "0123456789abcdef0123456789abcdef"

func TestMastraEmbedderSendsDirectProviderBatch(t *testing.T) {
	var payload map[string]any
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/vectorize", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "Bearer "+testInternalSecret, r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		_, _ = w.Write([]byte(`{"embeddings":[[1,2],[3,4]]}`))
	}))
	defer ai.Close()
	snapshot := &ChatCapability{
		Model:    model.ProviderModel{Kind: "embedding", ModelID: "bge-m3", Dimension: 2},
		Provider: model.ModelProvider{BaseURL: "https://api.example.com/v1", Protocol: "chat"},
		Key:      "sk-test",
	}
	vectors, err := (&MastraEmbedder{AIURL: ai.URL, Secret: testInternalSecret}).Embed(context.Background(), snapshot, []string{"第一段", "第二段"})
	require.NoError(t, err)
	require.Equal(t, [][]float64{{1, 2}, {3, 4}}, vectors)
	require.NotContains(t, payload, "gateway")
	require.NotContains(t, payload, "capability")
	require.Equal(t, float64(2), payload["dimension"])
	require.Equal(t, []any{"第一段", "第二段"}, payload["texts"])
	require.Equal(t, map[string]any{
		"protocol": "chat", "model_id": "bge-m3",
		"base_url": "https://api.example.com/v1", "api_key": "sk-test",
	}, payload["model"])
}

func TestMastraEmbedderRejectsMismatchedResponse(t *testing.T) {
	for name, body := range map[string]string{
		"count":     `{"embeddings":[[1,2]]}`,
		"dimension": `{"embeddings":[[1],[2]]}`,
		"zero":      `{"embeddings":[[0,0],[0,0]]}`,
	} {
		t.Run(name, func(t *testing.T) {
			ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer ai.Close()
			snapshot := &ChatCapability{
				Model:    model.ProviderModel{Kind: "embedding", ModelID: "bge-m3", Dimension: 2},
				Provider: model.ModelProvider{BaseURL: "https://api.example.com/v1", Protocol: "chat"},
			}
			_, err := (&MastraEmbedder{AIURL: ai.URL, Secret: testInternalSecret}).Embed(context.Background(), snapshot, []string{"第一段", "第二段"})
			require.Error(t, err)
		})
	}
}

// 400/403 是契约或配置错误：必须暴露成可判定的拒绝类型并带状态码，
// 否则调用方会当成瞬时故障退避重试，用户白等三分钟还看不到真实原因。
func TestMastraEmbedderClassifiesRejectedRequests(t *testing.T) {
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"Invalid vectorize input"}`))
	}))
	defer ai.Close()
	snapshot := &ChatCapability{
		Model:    model.ProviderModel{Kind: "embedding", ModelID: "embeddinggemma", Dimension: 2},
		Provider: model.ModelProvider{BaseURL: "http://127.0.0.1:11434/v1", Protocol: "chat"},
	}
	_, err := (&MastraEmbedder{AIURL: ai.URL, Secret: testInternalSecret}).Embed(context.Background(), snapshot, []string{"第一段"})
	var rejected *EmbeddingRejectedError
	require.ErrorAs(t, err, &rejected)
	require.Equal(t, http.StatusBadRequest, rejected.Status)
	require.Contains(t, err.Error(), "HTTP 400")
	require.Contains(t, err.Error(), "文本长度")
}

// 上游 5xx 仍按可重试错误返回，且文案里要能看到状态码。
func TestMastraEmbedderKeepsUpstreamFailureRetryable(t *testing.T) {
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer ai.Close()
	snapshot := &ChatCapability{
		Model:    model.ProviderModel{Kind: "embedding", ModelID: "embeddinggemma", Dimension: 2},
		Provider: model.ModelProvider{BaseURL: "http://127.0.0.1:11434/v1", Protocol: "chat"},
	}
	_, err := (&MastraEmbedder{AIURL: ai.URL, Secret: testInternalSecret}).Embed(context.Background(), snapshot, []string{"第一段"})
	var rejected *EmbeddingRejectedError
	require.False(t, errors.As(err, &rejected))
	require.Contains(t, err.Error(), "HTTP 502")
}

func TestMastraEmbedderRequiresInternalSecret(t *testing.T) {
	called := false
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ai.Close()
	snapshot := &ChatCapability{
		Model:    model.ProviderModel{Kind: "embedding", ModelID: "bge-m3", Dimension: 2},
		Provider: model.ModelProvider{BaseURL: "https://api.example.com/v1", Protocol: "chat"},
	}
	// 未配置共享密钥时必须在发出请求前失败，不能把未鉴权的调用打到 AI 服务
	_, err := (&MastraEmbedder{AIURL: ai.URL}).Embed(context.Background(), snapshot, []string{"第一段"})
	require.Error(t, err)
	require.False(t, called)
}
