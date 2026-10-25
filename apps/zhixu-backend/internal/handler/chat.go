package handler

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"zhixu-backend/internal/log"
	"zhixu-backend/internal/model"
	"zhixu-backend/internal/response"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	Service            *service.ChatService
	AIURL              string
	InternalBackendURL string
	InternalAPISecret  string
	Rag                *service.RagSearcher
	Conversations      *service.ConversationService
}

// newConversationID returns a random UUIDv4 for a fresh Mastra thread.
func newConversationID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	var buf [36]byte
	hex.Encode(buf[0:8], b[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], b[10:16])
	return string(buf[:])
}

func chatEvent(c *gin.Context, event any) {
	data, _ := json.Marshal(event)
	_, _ = c.Writer.Write(append(data, '\n'))
	c.Writer.Flush()
}

func firstUserMessage(messages []service.ChatMessage) string {
	for _, m := range messages {
		if m.Role == "user" {
			return strings.TrimSpace(m.Content)
		}
	}
	return ""
}

func truncateTitle(title string) string {
	title = strings.Join(strings.Fields(title), " ")
	runes := []rune(title)
	if len(runes) > 50 {
		return string(runes[:50])
	}
	return title
}

// Send godoc
// @Summary 临时无工具多轮对话，返回 NDJSON 流
// @Tags chat
// @Accept json
// @Produce application/x-ndjson
// @Param data body service.ChatInput true "用户模型 ID 与会话"
// @Success 200 {string} string "meta / delta / done / error 事件"
// @Security ApiKeyAuth
// @Router /chat [post]
func (h *ChatHandler) Send(c *gin.Context) {
	user, ok := knowledgeUser(c)
	if !ok {
		return
	}
	if strings.TrimSpace(h.InternalAPISecret) == "" {
		response.FailWithStatus(c, 503, response.CodeServerError, "AI 内部认证尚未配置")
		return
	}
	var input service.ChatInput
	if !bindAIConfig(c, &input) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()
	prepared, err := h.Service.Prepare(ctx, user, input)
	if err != nil {
		(&AIConfigHandler{}).fail(c, err)
		return
	}
	cap := prepared.Chat
	// RAG turns drive several model calls (rewrite + answer) through this token.
	cap.MultiStep = prepared.Rag && h.Rag != nil
	token, revoke, err := h.Service.Register(cap)
	if err != nil {
		response.FailWithStatus(c, 500, response.CodeServerError, "对话服务不可用")
		return
	}
	defer revoke()
	// Conversation mode persists this turn to the user's Mastra thread. Continue the
	// client-supplied thread, or open a fresh one whose title is seeded from the first
	// user message so the AI service never spends a model call generating one.
	conversationID := ""
	var conversation gin.H
	if h.Conversations != nil {
		conversationID = strings.TrimSpace(input.ConversationID)
		if conversationID == "" {
			conversationID = newConversationID()
		}
		if err := h.Conversations.EnsureActive(ctx, user, conversationID); err != nil {
			response.FailWithStatus(c, 409, response.CodeInvalidParam, "会话已删除或暂时不可用，请开始新对话")
			return
		}
		title := strings.TrimSpace(input.Title)
		if title == "" {
			title = firstUserMessage(input.Messages)
		}
		conversation = gin.H{
			"id": conversationID, "resource": service.ResourceID(user),
			"title": truncateTitle(title), "metadata": gin.H{"rag": prepared.Rag},
		}
	}
	payload, _ := json.Marshal(gin.H{
		"messages": input.Messages, "model": cap.Model.ModelID, "protocol": cap.Provider.Protocol,
		"gateway":      strings.TrimRight(h.InternalBackendURL, "/") + "/internal/chat/v1",
		"capability":   token,
		"conversation": conversation,
	})
	route := "/plain-chat"
	isRag := prepared.Rag && h.Rag != nil
	if isRag {
		ragToken, ragRevoke, ragErr := h.Service.Register(&service.ChatCapability{
			Context: ctx, RagQuery: &service.RagQuery{UserID: user},
		})
		if ragErr != nil {
			response.FailWithStatus(c, 500, response.CodeServerError, "对话服务不可用")
			return
		}
		defer ragRevoke()
		payload, _ = json.Marshal(gin.H{
			"messages": input.Messages, "model": cap.Model.ModelID, "protocol": cap.Provider.Protocol,
			"gateway":    strings.TrimRight(h.InternalBackendURL, "/") + "/internal/chat/v1",
			"capability": token,
			"rag": gin.H{
				"capability": ragToken,
				"search_url": strings.TrimRight(h.InternalBackendURL, "/") + "/internal/rag/v1/search",
				"top_k":      service.RagDefaultTopK,
			},
			"conversation": conversation,
		})
		route = "/rag-chat"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(h.AIURL, "/")+route, bytes.NewReader(payload))
	if err != nil {
		response.FailWithStatus(c, 503, response.CodeServerError, "AI 服务地址配置无效")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.InternalAPISecret)
	// No automatic retries: a retry could duplicate a paid model invocation.
	client := &http.Client{Transport: &http.Transport{Proxy: nil, ResponseHeaderTimeout: 15 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	upstream, err := client.Do(req)
	if err != nil {
		response.FailWithStatus(c, 503, response.CodeServerError, "AI 服务未启动或连接超时，请检查 Mastra 服务")
		return
	}
	defer upstream.Body.Close()
	if upstream.StatusCode != 200 || !strings.Contains(upstream.Header.Get("Content-Type"), "application/x-ndjson") {
		response.FailWithStatus(c, 503, response.CodeServerError, "AI 服务暂时不可用")
		return
	}
	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	c.Header("X-Accel-Buffering", "no")
	c.Status(200)
	chatEvent(c, gin.H{"type": "meta", "model_id": cap.Model.ID, "model": cap.Model.ModelID, "provider": cap.Provider.Name, "conversation_id": conversationID})
	// Relay the NDJSON stream line by line so RAG retrieval events can be captured for
	// later citation rendering while bytes still flow to the client unchanged.
	var ragQuery string
	var ragSources []model.KnowledgeSource
	scanner := bufio.NewScanner(upstream.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event struct {
			Type      string                  `json:"type"`
			MessageID string                  `json:"message_id"`
			Query     string                  `json:"query"`
			Items     []model.KnowledgeSource `json:"items"`
		}
		if json.Unmarshal(line, &event) == nil {
			switch event.Type {
			case "sources":
				ragQuery, ragSources = event.Query, event.Items
			case "done":
				// Save provenance before exposing completion, so immediately reopening
				// this thread cannot race the source write. Use the exact persisted ID.
				if isRag && h.Conversations != nil && conversationID != "" {
					saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
					err := h.Conversations.SaveTurnSource(saveCtx, conversationID, user, event.MessageID, ragQuery, ragSources)
					saveCancel()
					if err != nil {
						log.Error().Err(err).Msg("对话知识来源保存失败")
						chatEvent(c, gin.H{"type": "error", "message": "回复已生成，但知识来源未保存，请刷新后检查"})
						return
					}
				}
			}
		}
		if _, err := c.Writer.Write(append(line, '\n')); err != nil {
			return
		}
		c.Writer.Flush()
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		chatEvent(c, gin.H{"type": "error", "message": "回复连接中断，请手动重试"})
	}
}

// Gateway uses a short-lived, unguessable capability bound to the active authenticated request.
// It never exposes the provider key to Mastra, nor accepts arbitrary target URLs or models.
func (h *ChatHandler) Gateway(c *gin.Context) {
	cap := h.Service.ClaimCapability(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	if cap == nil {
		c.Status(404)
		return
	}
	if cap.Model.Kind == "embedding" {
		h.embeddingGateway(c, cap)
		return
	}
	path := strings.TrimPrefix(c.Param("endpoint"), "/")
	expected := "chat/completions"
	if cap.Provider.Protocol == "responses" {
		expected = "responses"
	}
	if path != expected {
		c.Status(404)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 512*1024)
	var body map[string]json.RawMessage
	if err := json.NewDecoder(c.Request.Body).Decode(&body); err != nil {
		c.Status(400)
		return
	}
	// Only language generation fields are forwarded; no tools or remote resources.
	clean := map[string]json.RawMessage{}
	for _, key := range []string{"messages", "input", "stream", "temperature", "top_p", "max_tokens", "max_output_tokens", "stream_options"} {
		if value, ok := body[key]; ok {
			clean[key] = value
		}
	}
	clean["model"], _ = json.Marshal(cap.Model.ModelID)
	clean["stream"] = json.RawMessage("true")
	if expected == "responses" {
		clean["store"] = json.RawMessage("false")
	}
	payload, _ := json.Marshal(clean)
	ctx, cancel := context.WithCancel(cap.Context)
	defer cancel()
	client, cleanup, err := h.Service.ModelClient(ctx, cap.Provider.BaseURL)
	if err != nil {
		c.JSON(403, gin.H{"error": gin.H{"message": "模型地址未获允许或无法解析"}})
		return
	}
	defer cleanup()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(cap.Provider.BaseURL, "/")+"/"+expected, bytes.NewReader(payload))
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
		c.JSON(504, gin.H{"error": gin.H{"message": "模型连接失败或超时"}})
		return
	}
	defer upstream.Body.Close()
	if upstream.StatusCode != 200 {
		// Never forward provider error bodies: they can contain credentials or request content.
		c.JSON(upstream.StatusCode, gin.H{"error": gin.H{"message": "模型供应商拒绝请求，请检查认证、地址、模型与额度"}})
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-store")
	c.Header("X-Accel-Buffering", "no")
	c.Status(200)
	buffer := make([]byte, 4096)
	for {
		n, err := upstream.Body.Read(buffer)
		if n > 0 {
			if _, e := c.Writer.Write(buffer[:n]); e != nil {
				return
			}
			c.Writer.Flush()
		}
		if err != nil {
			return
		}
	}
}
