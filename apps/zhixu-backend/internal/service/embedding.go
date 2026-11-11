package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// MastraEmbedder asks the AI service to turn a batch of texts into vectors. The
// configured embedding model travels with each request, so the AI service needs no
// credential store of its own and Go no longer proxies provider traffic.
// Secret is the internal bearer shared with the AI service: /vectorize rejects
// unauthenticated callers because they could steer outbound model requests.
type MastraEmbedder struct {
	AIURL  string
	Secret string
}

// EmbeddingRejectedError 表示 AI 服务拒绝了请求本身（HTTP 400/403）：文本过长或模型配置
// 不合规，重试只会重复同一次拒绝。调用方据此把任务直接判终态，不再空转退避重试。
type EmbeddingRejectedError struct{ Status int }

func (e *EmbeddingRejectedError) Error() string {
	return fmt.Sprintf("Embedding 请求被拒绝（HTTP %d），请检查文本长度与模型配置", e.Status)
}

func ValidateEmbeddings(vectors [][]float64, count, dimension int) error {
	if len(vectors) != count || count == 0 {
		return errors.New("embedding count mismatch")
	}
	for _, vector := range vectors {
		if len(vector) != dimension || dimension < 1 {
			return errors.New("embedding dimension mismatch")
		}
		norm := 0.0
		for _, value := range vector {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return errors.New("invalid embedding value")
			}
			norm += value * value
		}
		if norm == 0 || math.IsInf(norm, 0) {
			return errors.New("invalid embedding norm")
		}
	}
	return nil
}

func (e *MastraEmbedder) Embed(ctx context.Context, snapshot *ChatCapability, texts []string) ([][]float64, error) {
	if strings.TrimSpace(e.Secret) == "" {
		return nil, errors.New("INTERNAL_API_SECRET 未配置，无法调用 AI 服务")
	}
	payload, _ := json.Marshal(map[string]any{
		"model": map[string]any{
			"protocol": snapshot.Provider.Protocol, "model_id": snapshot.Model.ModelID,
			"base_url": snapshot.Provider.BaseURL, "api_key": snapshot.Key,
		},
		"dimension": snapshot.Model.Dimension,
		"texts":     texts,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(e.AIURL, "/")+"/vectorize", bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("AI 服务地址配置无效")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.Secret)
	client := &http.Client{
		Timeout:       105 * time.Second,
		Transport:     &http.Transport{Proxy: nil, ResponseHeaderTimeout: 100 * time.Second},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	response, err := client.Do(req)
	if err != nil {
		return nil, errors.New("Mastra 服务未启动或响应超时")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		// 带上状态码：没有它，排障时只能看到一句通用文案，无法区分契约错误与上游故障。
		switch response.StatusCode {
		case http.StatusBadRequest, http.StatusForbidden:
			return nil, &EmbeddingRejectedError{Status: response.StatusCode}
		default:
			return nil, fmt.Errorf("Embedding 调用失败（HTTP %d），请检查 Mastra、模型地址、凭据及额度", response.StatusCode)
		}
	}
	var result struct {
		Embeddings [][]float64 `json:"embeddings"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 16*1024*1024)).Decode(&result); err != nil {
		return nil, errors.New("AI 服务返回了无效的向量数据")
	}
	if err := ValidateEmbeddings(result.Embeddings, len(texts), int(snapshot.Model.Dimension)); err != nil {
		return nil, errors.New("Embedding 数量或维度与模型配置不一致")
	}
	return result.Embeddings, nil
}
