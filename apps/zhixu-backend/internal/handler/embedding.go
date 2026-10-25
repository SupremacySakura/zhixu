package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"

	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
)

func (h *ChatHandler) embeddingGateway(c *gin.Context, cap *service.ChatCapability) {
	if strings.TrimPrefix(c.Param("endpoint"), "/") != "embeddings" {
		c.Status(404)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128*1024)
	var body struct {
		Input []string `json:"input"`
	}
	if json.NewDecoder(c.Request.Body).Decode(&body) != nil || len(body.Input) == 0 || !reflect.DeepEqual(body.Input, cap.Texts) {
		c.Status(400)
		return
	}
	// Only the exact worker batch and configured model can use this one-time capability.
	payload, _ := json.Marshal(gin.H{"input": cap.Texts, "model": cap.Model.ModelID, "encoding_format": "float"})
	client, cleanup, err := h.Service.ModelClient(cap.Context, cap.Provider.BaseURL)
	if err != nil {
		c.JSON(403, gin.H{"error": gin.H{"message": "模型地址未获允许或无法解析"}})
		return
	}
	defer cleanup()
	req, err := http.NewRequestWithContext(cap.Context, http.MethodPost, strings.TrimRight(cap.Provider.BaseURL, "/")+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		c.Status(400)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if cap.Key != "" {
		req.Header.Set("Authorization", "Bearer "+cap.Key)
	}
	upstream, err := client.Do(req)
	if err != nil {
		c.JSON(504, gin.H{"error": gin.H{"message": "Embedding 连接失败或超时"}})
		return
	}
	defer upstream.Body.Close()
	if upstream.StatusCode != 200 {
		c.JSON(upstream.StatusCode, gin.H{"error": gin.H{"message": "Embedding 供应商拒绝请求"}})
		return
	}
	var result struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(upstream.Body, 16*1024*1024)).Decode(&result) != nil || len(result.Data) != len(cap.Texts) {
		c.Status(502)
		return
	}
	vectors := make([][]float64, len(cap.Texts))
	for _, row := range result.Data {
		if row.Index < 0 || row.Index >= len(vectors) || vectors[row.Index] != nil {
			c.Status(502)
			return
		}
		vectors[row.Index] = row.Embedding
	}
	if service.ValidateEmbeddings(vectors, len(cap.Texts), int(cap.Model.Dimension)) != nil {
		c.JSON(502, gin.H{"error": gin.H{"message": "Embedding 数量或维度与配置不一致"}})
		return
	}
	data := make([]gin.H, len(vectors))
	for i, vector := range vectors {
		data[i] = gin.H{"object": "embedding", "index": i, "embedding": vector}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"object": "list", "model": cap.Model.ModelID, "data": data, "usage": gin.H{"prompt_tokens": 0, "total_tokens": 0}})
}
