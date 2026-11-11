package handler

import (
	"errors"
	"net/http"
	"strconv"

	"zhixu-backend/internal/response"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
)

// RelationHandler 知识关系接口。
type RelationHandler struct {
	Service *service.RelationService
}

// List 返回某知识的已确认关系（GET /knowledge/:id/relations）。
func (h *RelationHandler) List(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	knowledgeID, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	list, err := h.Service.List(c.Request.Context(), userID, knowledgeID)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, list)
}

// Create 手动建立关系（POST /knowledge/:id/relations）。
func (h *RelationHandler) Create(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	knowledgeID, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	var req service.RelationCreateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithStatus(c, http.StatusBadRequest, response.CodeInvalidParam, "关系参数无效")
		return
	}
	view, err := h.Service.Create(c.Request.Context(), userID, knowledgeID, req)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.SuccessWithStatus(c, http.StatusCreated, view)
}

// Confirm 确认一条 AI 建议（POST /relations/:id/confirm）。
func (h *RelationHandler) Confirm(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	relationID, ok := relationIDParam(c, c.Param("id"))
	if !ok {
		return
	}
	view, err := h.Service.Confirm(c.Request.Context(), userID, relationID)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, view)
}

// Reject 忽略一条 AI 建议（POST /relations/:id/reject）。
func (h *RelationHandler) Reject(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	relationID, ok := relationIDParam(c, c.Param("id"))
	if !ok {
		return
	}
	view, err := h.Service.Reject(c.Request.Context(), userID, relationID)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, view)
}

// Update 修改已确认关系的类型（PUT /relations/:id）。
func (h *RelationHandler) Update(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	relationID, ok := relationIDParam(c, c.Param("id"))
	if !ok {
		return
	}
	var req service.RelationUpdateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithStatus(c, http.StatusBadRequest, response.CodeInvalidParam, "关系参数无效")
		return
	}
	view, err := h.Service.UpdateType(c.Request.Context(), userID, relationID, req.RelationType)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, view)
}

// Regenerate 手动触发一次关联推荐（POST /knowledge/:id/relations/regenerate）。
func (h *RelationHandler) Regenerate(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	knowledgeID, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	task, err := h.Service.Regenerate(c.Request.Context(), userID, knowledgeID)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.SuccessWithStatus(c, http.StatusAccepted, gin.H{"task": task})
}

// Delete 物理删除关系（DELETE /relations/:id）。
func (h *RelationHandler) Delete(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	relationID, ok := relationIDParam(c, c.Param("id"))
	if !ok {
		return
	}
	if err := h.Service.Delete(c.Request.Context(), userID, relationID); err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, nil)
}

// relationIDParam 解析关系 ID 参数。
func relationIDParam(c *gin.Context, raw string) (uint, bool) {
	id, err := strconv.ParseUint(raw, 10, strconv.IntSize)
	if err != nil || id == 0 {
		response.FailWithStatus(c, http.StatusBadRequest, response.CodeInvalidParam, "关系ID无效")
		return 0, false
	}
	return uint(id), true
}

// fail 统一关系接口错误响应。
func (h *RelationHandler) fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrRelationInvalid):
		response.FailWithStatus(c, http.StatusBadRequest, response.CodeInvalidParam, err.Error())
	case errors.Is(err, service.ErrRelationNotFound):
		response.FailWithStatus(c, http.StatusNotFound, response.CodeNotFound, "关系不存在")
	case errors.Is(err, service.ErrRelationConflict):
		response.FailWithStatus(c, http.StatusConflict, response.CodeConflict, err.Error())
	case errors.Is(err, service.ErrKnowledgeNotFound):
		response.FailWithStatus(c, http.StatusNotFound, response.CodeNotFound, "知识不存在")
	default:
		c.Error(err)
		response.FailWithStatus(c, http.StatusInternalServerError, response.CodeServerError, "关系服务暂时不可用")
	}
}
