package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"
	"zhixu-backend/internal/response"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
)

// EvolutionHandler 演化提醒与演化中心接口。
type EvolutionHandler struct {
	Service *service.EvolutionService
}

// List 分页查询演化提醒（GET /evolution/alerts）。
func (h *EvolutionHandler) List(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		h.fail(c, service.ErrEvolutionInvalid)
		return
	}
	size, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil {
		h.fail(c, service.ErrEvolutionInvalid)
		return
	}
	// open=true 表示只看待办视野（待处理 + 暂不处理），演化中心的两个 Tab 都靠它取数。
	open, ok := optionalBoolQuery(c, "open")
	if !ok {
		h.fail(c, service.ErrEvolutionInvalid)
		return
	}
	statuses, ok := evolutionStatusQuery(c.Query("status"))
	if !ok {
		h.fail(c, service.ErrEvolutionInvalid)
		return
	}
	views, total, err := h.Service.List(c.Request.Context(), userID, page, size,
		model.EvolutionAlertType(c.Query("type")), statuses, open != nil && *open)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, gin.H{"items": views, "total": total, "page": page, "page_size": size})
}

// evolutionStatusQuery 解析逗号分隔的状态筛选：空串表示不筛选，
// 单个值等价于等值匹配，多个值用于「处理记录」页签同时取已处理与已忽略。
// 这里只做拆分与去空，合法性由服务层统一校验（未知状态返回 400）。
func evolutionStatusQuery(raw string) ([]model.EvolutionAlertStatus, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	parts := strings.Split(raw, ",")
	statuses := make([]model.EvolutionAlertStatus, 0, len(parts))
	seen := make(map[model.EvolutionAlertStatus]bool, len(parts))
	for _, part := range parts {
		status := model.EvolutionAlertStatus(strings.TrimSpace(part))
		if status == "" || seen[status] {
			continue
		}
		seen[status] = true
		statuses = append(statuses, status)
	}
	if len(statuses) == 0 {
		return nil, false
	}
	return statuses, true
}

// Summary 返回待处理计数（GET /evolution/alerts/summary），首页卡片只调这一个接口。
func (h *EvolutionHandler) Summary(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	view, err := h.Service.Summary(c.Request.Context(), userID)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, view)
}

// evolutionResolveRequest 是裁决请求体；base_status 可选，用于乐观校验。
type evolutionResolveRequest struct {
	Action     model.EvolutionAction      `json:"action"`
	Note       string                     `json:"note"`
	BaseStatus model.EvolutionAlertStatus `json:"base_status"`
}

// Resolve 裁决一条提醒（POST /evolution/alerts/:id/resolve）。
// 已处理过的提醒再次提交按幂等处理：返回当前提醒、不重复执行动作。
func (h *EvolutionHandler) Resolve(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	alertID, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	var req evolutionResolveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithStatus(c, http.StatusBadRequest, response.CodeInvalidParam, "裁决参数无效")
		return
	}
	view, _, err := h.Service.Resolve(c.Request.Context(), userID, alertID, service.EvolutionResolveInput{
		Action: req.Action, Note: req.Note, BaseStatus: req.BaseStatus,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, view)
}

// Regenerate 手动重跑一条知识的演化检查（POST /knowledge/:id/evolve/regenerate）。
func (h *EvolutionHandler) Regenerate(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	kid, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	view, err := h.Service.Regenerate(c.Request.Context(), userID, kid)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.SuccessWithStatus(c, http.StatusAccepted, view)
}

// fail 统一演化接口错误响应。
func (h *EvolutionHandler) fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrEvolutionInvalid):
		response.FailWithStatus(c, http.StatusBadRequest, response.CodeInvalidParam, err.Error())
	case errors.Is(err, service.ErrEvolutionNotFound):
		response.FailWithStatus(c, http.StatusNotFound, response.CodeNotFound, "演化提醒不存在")
	case errors.Is(err, service.ErrEvolutionUnsupported):
		response.FailWithStatus(c, http.StatusConflict, response.CodeConflict, err.Error())
	case errors.Is(err, repository.ErrEvolutionConflict):
		// 期望状态校验失败：另一个标签页已经裁决过这条提醒，让前端刷新而不是当成服务故障。
		response.FailWithStatus(c, http.StatusConflict, response.CodeConflict, "提醒状态已变化，请刷新后重试")
	case errors.Is(err, service.ErrKnowledgeNotFound):
		response.FailWithStatus(c, http.StatusNotFound, response.CodeNotFound, "知识不存在")
	default:
		c.Error(err)
		response.FailWithStatus(c, http.StatusInternalServerError, response.CodeServerError, "演化服务暂时不可用")
	}
}
