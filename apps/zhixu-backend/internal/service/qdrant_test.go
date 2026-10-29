package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQdrantCreatesCollectionAndAcknowledgesUpsert(t *testing.T) {
	collectionReads, created := 0, false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/zhixu_u7_v3_c5":
			collectionReads++
			if !created {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"config": map[string]any{
				"params": map[string]any{"vectors": map[string]any{"size": 2, "distance": "Cosine"}},
			}}})
		case r.Method == http.MethodPut && r.URL.Path == "/collections/zhixu_u7_v3_c5":
			var body struct {
				Vectors struct {
					Size     uint   `json:"size"`
					Distance string `json:"distance"`
				} `json:"vectors"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, uint(2), body.Vectors.Size)
			require.Equal(t, "Cosine", body.Vectors.Distance)
			created = true
			_, _ = w.Write([]byte(`{"result":true}`))
		case r.Method == http.MethodPut && r.URL.Path == "/collections/zhixu_u7_v3_c5/points":
			require.Equal(t, "true", r.URL.Query().Get("wait"))
			var body struct {
				Points []struct {
					ID      uint      `json:"id"`
					Vector  []float64 `json:"vector"`
					Payload struct {
						ChunkID uint `json:"chunk_id"`
					} `json:"payload"`
				} `json:"points"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Len(t, body.Points, 1)
			require.Equal(t, uint(4), body.Points[0].ID)
			require.Equal(t, uint(4), body.Points[0].Payload.ChunkID)
			_, _ = w.Write([]byte(`{"result":{"status":"completed"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := &QdrantClient{URL: server.URL}
	require.NoError(t, client.EnsureCollection(context.Background(), "zhixu_u7_v3_c5", 2))
	require.Equal(t, 2, collectionReads)
	require.NoError(t, client.Upsert(context.Background(), "zhixu_u7_v3_c5", []VectorPoint{
		{ID: 4, Vector: []float64{1, 0}, Payload: map[string]any{"chunk_id": uint(4)}},
	}))
}

func TestQdrantRejectsDimensionMismatchAndUnconfirmedWrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"result":{"config":{"params":{"vectors":{"size":3,"distance":"Cosine"}}}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{"status":"acknowledged"}}`))
	}))
	defer server.Close()
	client := &QdrantClient{URL: server.URL}
	require.ErrorContains(t, client.EnsureCollection(context.Background(), "zhixu_u7_v3_c5", 2), "维度或距离度量")
	require.ErrorContains(t, client.Upsert(context.Background(), "zhixu_u7_v3_c5", []VectorPoint{{ID: 1, Vector: []float64{1}}}), "未确认")
}

func TestQdrantSearchFiltersUserAndReadsMatches(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/collections/zhixu_u7_v3_c5/points/search", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_ = json.NewEncoder(w).Encode(map[string]any{"result": []map[string]any{
			{"id": 11, "score": 0.9, "payload": map[string]any{"user_id": 7, "chunk_id": 11}},
			{"id": 12, "score": 0.4, "payload": map[string]any{"user_id": 8, "chunk_id": 12}},
			{"id": 13, "score": 0.8, "payload": map[string]any{"user_id": 7, "chunk_id": 13}},
		}})
	}))
	defer server.Close()
	client := &QdrantClient{URL: server.URL}
	matches, err := client.Search(context.Background(), "zhixu_u7_v3_c5", 7, []float64{1, 0}, 5, 0.5)
	require.NoError(t, err)
	require.Len(t, matches, 2)
	require.Equal(t, uint(11), matches[0].ChunkID)
	require.InDelta(t, 0.9, matches[0].Score, 1e-9)
	require.Equal(t, uint(13), matches[1].ChunkID)
	_, ok := body["filter"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, body["with_payload"])
	require.Equal(t, float64(5), body["limit"])
	require.Equal(t, 0.5, body["score_threshold"])
}

func TestQdrantCleanupFiltersUserKnowledgeAndAcknowledges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/collections" {
			_, _ = w.Write([]byte(`{"result":{"collections":[{"name":"zhixu_u7_v3"},{"name":"zhixu_u7_v3_c5"},{"name":"zhixu_u70_v3"},{"name":"other"}]}}`))
			return
		}
		require.Equal(t, "/collections/zhixu_u7_v3/points/delete", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "true", r.URL.Query().Get("wait"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		must := body["filter"].(map[string]any)["must"].([]any)
		require.Len(t, must, 2)
		require.Equal(t, "user_id", must[0].(map[string]any)["key"])
		require.Equal(t, float64(7), must[0].(map[string]any)["match"].(map[string]any)["value"])
		require.Equal(t, "knowledge_id", must[1].(map[string]any)["key"])
		require.Equal(t, float64(9), must[1].(map[string]any)["match"].(map[string]any)["value"])
		_, _ = w.Write([]byte(`{"result":{"status":"completed"}}`))
	}))
	defer server.Close()
	client := &QdrantClient{URL: server.URL}
	names, err := client.UserCollections(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, []string{"zhixu_u7_v3", "zhixu_u7_v3_c5"}, names)
	require.NoError(t, client.DeleteKnowledge(context.Background(), names[0], 7, 9))
}

func TestQdrantDeleteMissingCollectionAndUnconfirmed(t *testing.T) {
	status := http.StatusNotFound
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"result":{"status":"acknowledged"}}`))
	}))
	defer server.Close()
	client := &QdrantClient{URL: server.URL}
	require.NoError(t, client.DeleteKnowledge(context.Background(), "missing", 7, 9))
	status = http.StatusOK
	require.ErrorContains(t, client.DeleteKnowledge(context.Background(), "missing", 7, 9), "未确认删除")
}

func TestQdrantSearchMissingCollectionIsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	client := &QdrantClient{URL: server.URL}
	matches, err := client.Search(context.Background(), "missing", 7, []float64{1}, 5, 0.5)
	require.NoError(t, err)
	require.Empty(t, matches)
}
