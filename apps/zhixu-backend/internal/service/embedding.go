package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

type MastraEmbedder struct {
	Chat       *ChatService
	AIURL      string
	BackendURL string
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
	cap := &ChatCapability{Model: snapshot.Model, Provider: snapshot.Provider, Key: snapshot.Key, Context: ctx, Texts: texts}
	token, revoke, err := e.Chat.Register(cap)
	if err != nil {
		return nil, err
	}
	defer revoke()
	payload, _ := json.Marshal(map[string]any{
		"model": cap.Model.ModelID, "dimension": cap.Model.Dimension, "texts": texts,
		"gateway": strings.TrimRight(e.BackendURL, "/") + "/internal/chat/v1", "capability": token,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(e.AIURL, "/")+"/vectorize", bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("AI 服务地址配置无效")
	}
	req.Header.Set("Content-Type", "application/json")
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
		return nil, errors.New("Embedding 调用失败，请检查 Mastra、模型地址、凭据及额度")
	}
	var result struct {
		Embeddings [][]float64 `json:"embeddings"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 16*1024*1024)).Decode(&result); err != nil {
		return nil, errors.New("AI 服务返回了无效的向量数据")
	}
	if err := ValidateEmbeddings(result.Embeddings, len(texts), int(cap.Model.Dimension)); err != nil {
		return nil, errors.New("Embedding 数量或维度与模型配置不一致")
	}
	return result.Embeddings, nil
}
