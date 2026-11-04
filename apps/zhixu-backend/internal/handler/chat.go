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
// @Summary 知识库对话，模型自主决定是否检索，返回 NDJSON 流
// @Tags chat
// @Accept json
// @Produce application/x-ndjson
// @Param data body service.ChatInput true "用户模型 ID 与会话"
// @Success 200 {string} string "meta / delta / sources / done / error 事件"
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
	// Threads resumed from history can carry assistant turns that produced no visible
	// text (interrupted multi-step answers). Drop them before validating the shape.
	input.Messages = service.NormalizeChatHistory(input.Messages)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()
	chatCap, err := h.Service.Resolve(ctx, user, input)
	if err != nil {
		(&AIConfigHandler{}).fail(c, err)
		return
	}
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
			"title": truncateTitle(title),
		}
	}
	if h.Rag == nil {
		response.FailWithStatus(c, 503, response.CodeServerError, "检索服务尚未配置")
		return
	}
	// Retrieval is a tool owned by the AI service: the model decides when to search.
	// This short-lived, read-only token authorizes that tool for this turn only.
	token, revoke, err := h.Service.Register(&service.ChatCapability{
		Context: ctx, RagQuery: &service.RagQuery{UserID: user},
	})
	if err != nil {
		response.FailWithStatus(c, 500, response.CodeServerError, "对话服务不可用")
		return
	}
	defer revoke()
	// The AI service calls the provider directly; Go only issues the tool capability
	// and the per-turn model configuration, so provider keys never leave this process
	// unencrypted at rest.
	payload, _ := json.Marshal(gin.H{
		"version": 1,
		"model": gin.H{
			"protocol": chatCap.Provider.Protocol, "model_id": chatCap.Model.ModelID,
			"base_url": chatCap.Provider.BaseURL, "api_key": chatCap.Key,
		},
		"messages": input.Messages, "conversation": conversation,
		"tools": gin.H{
			"search_url": strings.TrimRight(h.InternalBackendURL, "/") + "/internal/tools/v1/search",
			"capability": token,
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(h.AIURL, "/")+"/chat", bytes.NewReader(payload))
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
	chatEvent(c, gin.H{"type": "meta", "model_id": chatCap.Model.ID, "model": chatCap.Model.ModelID, "provider": chatCap.Provider.Name, "conversation_id": conversationID})
	// Relay the NDJSON stream line by line so tool retrieval events can be captured for
	// later citation rendering while bytes still flow to the client unchanged.
	var toolQuery string
	var toolSources []model.KnowledgeSource
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
			Usage     *struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
				TotalTokens  int `json:"total_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(line, &event) == nil {
			switch event.Type {
			case "sources":
				toolQuery, toolSources = event.Query, event.Items
			case "done":
				// Save provenance before exposing completion, so immediately reopening
				// this thread cannot race the source write. Use the exact persisted ID.
				if len(toolSources) > 0 && h.Conversations != nil && conversationID != "" {
					saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
					err := h.Conversations.SaveTurnSource(saveCtx, conversationID, user, event.MessageID, toolQuery, toolSources)
					saveCancel()
					if err != nil {
						log.Error().Err(err).Msg("对话知识来源保存失败")
						chatEvent(c, gin.H{"type": "error", "message": "回复已生成，但知识来源未保存，请刷新后检查"})
						return
					}
				}
				// Token usage is accounting only: a failed ledger write must never
				// withhold a reply the user has already paid for and received.
				if event.Usage != nil && h.Conversations != nil && conversationID != "" {
					saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
					err := h.Conversations.SaveTurnUsage(saveCtx, &model.ChatTurnUsage{
						UserID: user, ConversationID: conversationID, MessageID: event.MessageID,
						ModelID: chatCap.Model.ID, Model: chatCap.Model.ModelID,
						InputTokens: event.Usage.InputTokens, OutputTokens: event.Usage.OutputTokens,
						TotalTokens: event.Usage.TotalTokens,
					})
					saveCancel()
					if err != nil {
						log.Error().Err(err).Msg("对话用量记录失败")
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
