package handler

import (
	"context"
	"errors"
	"net/http"

	"zhixu-backend/internal/response"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
)

// GraphReader 是图谱接口依赖的服务能力（*service.GraphService 满足）。
type GraphReader interface {
	Graph(context.Context, uint, service.GraphQuery) (service.GraphView, error)
}

// GraphHandler 局部图谱接口。
type GraphHandler struct{ Service GraphReader }

// Get 返回当前用户的局部图谱（GET /knowledge/graph）。
// 参数：center_id 可选；depth 本批固定 1；include_suggested 0/1，默认 0。
func (h *GraphHandler) Get(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	centerID, ok := optionalUintQuery(c, "center_id")
	if !ok || (c.Query("center_id") != "" && centerID == 0) {
		response.FailWithStatus(c, http.StatusBadRequest, response.CodeInvalidParam, "中心知识 ID 无效")
		return
	}
	if depth := c.Query("depth"); depth != "" && depth != "1" {
		response.FailWithStatus(c, http.StatusBadRequest, response.CodeInvalidParam, "图谱深度仅支持 1")
		return
	}
	includeSuggested := false
	if flag, ok := optionalBoolQuery(c, "include_suggested"); !ok {
		response.FailWithStatus(c, http.StatusBadRequest, response.CodeInvalidParam, "建议边参数无效")
		return
	} else if flag != nil {
		includeSuggested = *flag
	}
	view, err := h.Service.Graph(c.Request.Context(), userID, service.GraphQuery{
		CenterID: centerID, IncludeSuggested: includeSuggested,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrKnowledgeNotFound):
			response.FailWithStatus(c, http.StatusNotFound, response.CodeNotFound, "知识不存在或已归档")
		default:
			response.FailWithStatus(c, http.StatusInternalServerError, response.CodeServerError, "图谱服务暂时不可用")
		}
		return
	}
	response.Success(c, view)
}
