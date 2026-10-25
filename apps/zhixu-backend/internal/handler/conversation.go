package handler

import (
	"strings"

	"zhixu-backend/internal/response"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
)

// ConversationHandler exposes per-user chat history. Every route requires the JWT
// middleware and scopes all access to the authenticated user; the AI service enforces
// the same scope again via the resource id it receives from here.
type ConversationHandler struct {
	Conversations *service.ConversationService
}

// List godoc
// @Summary 会话列表
// @Tags chat
// @Produce json
// @Success 200 {object} response.Response
// @Security ApiKeyAuth
// @Router /chat/conversations [get]
func (h *ConversationHandler) List(c *gin.Context) {
	user, ok := knowledgeUser(c)
	if !ok {
		return
	}
	conversations, err := h.Conversations.List(c.Request.Context(), user)
	if err != nil {
		response.FailWithStatus(c, 503, response.CodeServerError, "会话服务暂时不可用")
		return
	}
	response.Success(c, gin.H{"conversations": conversations})
}

// Detail godoc
// @Summary 会话详情（含消息与知识来源）
// @Tags chat
// @Produce json
// @Param id path string true "会话 ID"
// @Success 200 {object} response.Response
// @Security ApiKeyAuth
// @Router /chat/conversations/{id} [get]
func (h *ConversationHandler) Detail(c *gin.Context) {
	user, ok := knowledgeUser(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" || len(id) > 64 {
		response.FailWithStatus(c, 400, response.CodeInvalidParam, "会话 ID 无效")
		return
	}
	detail, err := h.Conversations.Detail(c.Request.Context(), user, id)
	if err != nil {
		response.FailWithStatus(c, 404, response.CodeNotFound, "会话不存在")
		return
	}
	response.Success(c, detail)
}

type conversationRenameInput struct {
	Title string `json:"title" binding:"required,max=200"`
}

// Rename godoc
// @Summary 重命名会话
// @Tags chat
// @Accept json
// @Produce json
// @Param id path string true "会话 ID"
// @Param data body conversationRenameInput true "新标题"
// @Success 200 {object} response.Response
// @Security ApiKeyAuth
// @Router /chat/conversations/{id} [patch]
func (h *ConversationHandler) Rename(c *gin.Context) {
	user, ok := knowledgeUser(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" || len(id) > 64 {
		response.FailWithStatus(c, 400, response.CodeInvalidParam, "会话 ID 无效")
		return
	}
	var input conversationRenameInput
	if !bindAIConfig(c, &input) {
		return
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		response.FailWithStatus(c, 400, response.CodeInvalidParam, "标题不能为空")
		return
	}
	if err := h.Conversations.Rename(c.Request.Context(), user, id, title); err != nil {
		response.FailWithStatus(c, 404, response.CodeNotFound, "会话不存在")
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// Delete godoc
// @Summary 删除会话
// @Tags chat
// @Produce json
// @Param id path string true "会话 ID"
// @Success 200 {object} response.Response
// @Security ApiKeyAuth
// @Router /chat/conversations/{id} [delete]
func (h *ConversationHandler) Delete(c *gin.Context) {
	user, ok := knowledgeUser(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" || len(id) > 64 {
		response.FailWithStatus(c, 400, response.CodeInvalidParam, "会话 ID 无效")
		return
	}
	if err := h.Conversations.Delete(c.Request.Context(), user, id); err != nil {
		response.FailWithStatus(c, 404, response.CodeNotFound, "会话不存在")
		return
	}
	response.Success(c, gin.H{"ok": true})
}
