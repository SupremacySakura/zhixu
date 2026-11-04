package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"zhixu-backend/internal/model"

	"github.com/stretchr/testify/require"
)

func TestMastraEmbedderSendsDirectProviderBatch(t *testing.T) {
	var payload map[string]any
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/vectorize", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		_, _ = w.Write([]byte(`{"embeddings":[[1,2],[3,4]]}`))
	}))
	defer ai.Close()
	snapshot := &ChatCapability{
		Model:    model.ProviderModel{Kind: "embedding", ModelID: "bge-m3", Dimension: 2},
		Provider: model.ModelProvider{BaseURL: "https://api.example.com/v1", Protocol: "chat"},
		Key:      "sk-test",
	}
	vectors, err := (&MastraEmbedder{AIURL: ai.URL}).Embed(context.Background(), snapshot, []string{"第一段", "第二段"})
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
			_, err := (&MastraEmbedder{AIURL: ai.URL}).Embed(context.Background(), snapshot, []string{"第一段", "第二段"})
			require.Error(t, err)
		})
	}
}
