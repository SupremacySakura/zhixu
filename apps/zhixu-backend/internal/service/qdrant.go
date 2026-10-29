package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type VectorPoint struct {
	ID      uint           `json:"id"`
	Vector  []float64      `json:"vector"`
	Payload map[string]any `json:"payload"`
}

type QdrantClient struct {
	URL    string
	APIKey string
}

type VectorMatch struct {
	ChunkID uint
	Score   float64
}

func (q *QdrantClient) request(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	if q.URL == "" {
		return nil, 0, errors.New("Qdrant 地址尚未配置")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(q.URL, "/")+path, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, errors.New("Qdrant 地址无效")
	}
	req.Header.Set("Content-Type", "application/json")
	if q.APIKey != "" {
		req.Header.Set("api-key", q.APIKey)
	}
	client := &http.Client{
		Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	response, err := client.Do(req)
	if err != nil {
		return nil, 0, errors.New("无法连接 Qdrant，请检查原生服务或部署配置")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return nil, response.StatusCode, errors.New("读取 Qdrant 响应失败")
	}
	return data, response.StatusCode, nil
}

func (q *QdrantClient) EnsureCollection(ctx context.Context, name string, dimension uint) error {
	data, status, err := q.request(ctx, "GET", "/collections/"+name, nil)
	if err != nil {
		return err
	}
	if status == 404 {
		_, status, err = q.request(ctx, "PUT", "/collections/"+name, map[string]any{
			"vectors": map[string]any{"size": dimension, "distance": "Cosine"},
		})
		if err != nil {
			return err
		}
		// Another worker may create the same collection; always verify its dimensions.
		if status != 200 && status != 409 {
			return fmt.Errorf("Qdrant 创建集合失败（HTTP %d）", status)
		}
		data, status, err = q.request(ctx, "GET", "/collections/"+name, nil)
	}
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("Qdrant 集合不可用（HTTP %d）", status)
	}
	var collection struct {
		Result struct {
			Config struct {
				Params struct {
					Vectors struct {
						Size     uint   `json:"size"`
						Distance string `json:"distance"`
					} `json:"vectors"`
				} `json:"params"`
			} `json:"config"`
		} `json:"result"`
	}
	if json.Unmarshal(data, &collection) != nil || collection.Result.Config.Params.Vectors.Size != dimension ||
		collection.Result.Config.Params.Vectors.Distance != "Cosine" {
		return errors.New("Qdrant 集合维度或距离度量与模型配置不一致")
	}
	return nil
}

func (q *QdrantClient) Upsert(ctx context.Context, collection string, points []VectorPoint) error {
	data, status, err := q.request(ctx, "PUT", "/collections/"+collection+"/points?wait=true", map[string]any{"points": points})
	if err != nil {
		return err
	}
	var result struct {
		Result struct {
			Status string `json:"status"`
		} `json:"result"`
	}
	if status != 200 || json.Unmarshal(data, &result) != nil || result.Result.Status != "completed" {
		return fmt.Errorf("Qdrant 未确认分块写入完成（HTTP %d）", status)
	}
	return nil
}

// UserCollections lists only this user's canonical and legacy collection names.
func (q *QdrantClient) UserCollections(ctx context.Context, user uint) ([]string, error) {
	data, status, err := q.request(ctx, "GET", "/collections", nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Result struct {
			Collections []struct {
				Name string `json:"name"`
			} `json:"collections"`
		} `json:"result"`
	}
	if status != 200 || json.Unmarshal(data, &result) != nil {
		return nil, fmt.Errorf("Qdrant 集合列表失败（HTTP %d）", status)
	}
	pattern := regexp.MustCompile(fmt.Sprintf(`^zhixu_u%d_v[0-9]+(_c[0-9]+)?$`, user))
	names := []string{}
	for _, collection := range result.Result.Collections {
		if pattern.MatchString(collection.Name) {
			names = append(names, collection.Name)
		}
	}
	return names, nil
}

func (q *QdrantClient) DeleteKnowledge(ctx context.Context, collection string, user, knowledge uint) error {
	data, status, err := q.request(ctx, "POST", "/collections/"+collection+"/points/delete?wait=true", map[string]any{
		"filter": map[string]any{"must": []any{
			map[string]any{"key": "user_id", "match": map[string]any{"value": user}},
			map[string]any{"key": "knowledge_id", "match": map[string]any{"value": knowledge}},
		}},
	})
	if err != nil {
		return err
	}
	if status == 404 {
		return nil
	}
	var result struct {
		Result struct {
			Status string `json:"status"`
		} `json:"result"`
	}
	if status != 200 || json.Unmarshal(data, &result) != nil || result.Result.Status != "completed" {
		return fmt.Errorf("Qdrant 未确认删除完成（HTTP %d）", status)
	}
	return nil
}

// Search returns chunk matches scoped to one user's collection. The caller must ensure
// the collection name embeds that user id; the payload filter is defense in depth.
// minScore maps to Qdrant score_threshold so weak matches never leave the vector store.
func (q *QdrantClient) Search(ctx context.Context, collection string, user uint, vector []float64, topK int, minScore float64) ([]VectorMatch, error) {
	if topK < 1 || topK > RagMaxCandidates {
		return nil, errors.New("检索数量超出允许范围")
	}
	data, status, err := q.request(ctx, "POST", "/collections/"+collection+"/points/search", map[string]any{
		"vector": vector, "limit": topK, "with_payload": true, "score_threshold": minScore,
		"filter": map[string]any{"must": []any{
			map[string]any{"key": "user_id", "match": map[string]any{"value": user}},
		}},
	})
	if err != nil {
		return nil, err
	}
	if status == 404 {
		return nil, nil
	}
	var result struct {
		Result []struct {
			ID      uint    `json:"id"`
			Score   float64 `json:"score"`
			Payload struct {
				UserID  uint `json:"user_id"`
				ChunkID uint `json:"chunk_id"`
			} `json:"payload"`
		} `json:"result"`
	}
	if status != 200 || json.Unmarshal(data, &result) != nil {
		return nil, fmt.Errorf("Qdrant 检索失败（HTTP %d）", status)
	}
	matches := make([]VectorMatch, 0, len(result.Result))
	for _, point := range result.Result {
		if point.Payload.UserID != user || point.Payload.ChunkID == 0 {
			continue
		}
		matches = append(matches, VectorMatch{ChunkID: point.Payload.ChunkID, Score: point.Score})
	}
	return matches, nil
}
