package handler

import (
	"errors"
	"net/http"
	"strconv"

	"zhixu-backend/internal/response"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
)

// TagHandler 标签接口。
type TagHandler struct {
	Service *service.TagService
}

// TagRequest 创建/重命名标签请求。
type TagRequest struct {
	Name string `json:"name"`
}

// ReplaceKnowledgeTagsRequest 全量替换知识标签请求。
type ReplaceKnowledgeTagsRequest struct {
	Tags []string `json:"tags"`
}

// List 返回当前用户的全部标签及引用次数（GET /tags）。
func (h *TagHandler) List(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	items, err := h.Service.List(c.Request.Context(), userID)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

// Create 创建标签（POST /tags）；同名标签直接返回既有标签。
func (h *TagHandler) Create(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	var req TagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, service.ErrTagInvalid)
		return
	}
	view, err := h.Service.Create(c.Request.Context(), userID, req.Name)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.SuccessWithStatus(c, http.StatusCreated, view)
}

// Rename 重命名标签（PUT /tags/:id）。
func (h *TagHandler) Rename(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	id, ok := tagID(c, c.Param("id"))
	if !ok {
		return
	}
	var req TagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, service.ErrTagInvalid)
		return
	}
	view, err := h.Service.Rename(c.Request.Context(), userID, id, req.Name)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, view)
}

// Delete 删除标签（DELETE /tags/:id），同时解除全部知识绑定。
func (h *TagHandler) Delete(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	id, ok := tagID(c, c.Param("id"))
	if !ok {
		return
	}
	if err := h.Service.Delete(c.Request.Context(), userID, id); err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, gin.H{"id": id})
}

// KnowledgeTags 返回知识当前绑定的标签（GET /knowledge/:id/tags）。
func (h *TagHandler) KnowledgeTags(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	knowledgeID, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	items, err := h.Service.TagsForKnowledge(c.Request.Context(), userID, knowledgeID)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

// ReplaceKnowledgeTags 全量替换知识标签（PUT /knowledge/:id/tags）。
func (h *TagHandler) ReplaceKnowledgeTags(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	knowledgeID, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	var req ReplaceKnowledgeTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.fail(c, service.ErrTagInvalid)
		return
	}
	items, err := h.Service.ReplaceForKnowledge(c.Request.Context(), userID, knowledgeID, req.Tags)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

// tagID 解析标签 ID 参数。
func tagID(c *gin.Context, raw string) (uint, bool) {
	id, err := strconv.ParseUint(raw, 10, strconv.IntSize)
	if err != nil || id == 0 {
		response.FailWithStatus(c, http.StatusBadRequest, response.CodeInvalidParam, "标签ID无效")
		return 0, false
	}
	return uint(id), true
}

// fail 统一标签接口错误响应。
func (h *TagHandler) fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTagInvalid):
		response.FailWithStatus(c, http.StatusBadRequest, response.CodeInvalidParam, err.Error())
	case errors.Is(err, service.ErrTagNotFound):
		response.FailWithStatus(c, http.StatusNotFound, response.CodeNotFound, "标签不存在")
	case errors.Is(err, service.ErrTagConflict):
		response.FailWithStatus(c, http.StatusConflict, response.CodeConflict, "标签名称已存在")
	case errors.Is(err, service.ErrKnowledgeNotFound):
		response.FailWithStatus(c, http.StatusNotFound, response.CodeNotFound, "知识不存在")
	default:
		c.Error(err)
		response.FailWithStatus(c, http.StatusInternalServerError, response.CodeServerError, "标签服务暂时不可用")
	}
}
