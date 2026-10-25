package handler

import (
	"errors"
	"net/http"
	"strconv"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"
	"zhixu-backend/internal/response"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
)

type KnowledgeHandler struct{ Service *service.KnowledgeService }

type AddKnowledgeRequest struct {
	FileID uint   `json:"file_id" binding:"required"`
	Title  string `json:"title"`
}

// Delete godoc
// @Summary 删除知识条目，保留原文件
// @Tags knowledge
// @Param id path int true "知识ID"
// @Success 200 {object} response.Response
// @Security ApiKeyAuth
// @Router /knowledge/{id} [delete]
func (h *KnowledgeHandler) Delete(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	id, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	if err := h.Service.Delete(c.Request.Context(), userID, id); err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *KnowledgeHandler) fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrKnowledgeInvalid):
		response.FailWithStatus(c, http.StatusBadRequest, response.CodeInvalidParam, err.Error())
	case errors.Is(err, service.ErrKnowledgeNotFound):
		response.FailWithStatus(c, http.StatusNotFound, response.CodeNotFound, "知识或原文件不存在")
	case errors.Is(err, repository.ErrKnowledgeConflict):
		response.FailWithStatus(c, http.StatusConflict, response.CodeConflict, "当前状态不支持重试")
	default:
		c.Error(err)
		response.FailWithStatus(c, http.StatusInternalServerError, response.CodeServerError, "知识库服务暂时不可用")
	}
}

func knowledgeUser(c *gin.Context) (uint, bool) {
	id, ok := getCurrentUserID(c)
	if !ok || id == 0 {
		response.FailWithStatus(c, 401, response.CodeUnauthorized, "未登录")
		return 0, false
	}
	return id, true
}

func knowledgeID(c *gin.Context, raw string) (uint, bool) {
	id, err := strconv.ParseUint(raw, 10, strconv.IntSize)
	if err != nil || id == 0 {
		response.FailWithStatus(c, 400, response.CodeInvalidParam, "ID 参数无效")
		return 0, false
	}
	return uint(id), true
}

// Add godoc
// @Summary 将 Markdown 文件加入知识库
// @Description 首次加入返回202。重复请求返回原知识条目，不改变标题或重试失败任务。
// @Tags knowledge
// @Accept json
// @Produce json
// @Param data body AddKnowledgeRequest true "文件与可选标题"
// @Success 202 {object} response.Response{data=model.KnowledgeItem}
// @Success 200 {object} response.Response{data=model.KnowledgeItem}
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Security ApiKeyAuth
// @Router /knowledge [post]
func (h *KnowledgeHandler) Add(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	var req AddKnowledgeRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024)
	if c.ShouldBindJSON(&req) != nil {
		response.FailWithStatus(c, 400, response.CodeInvalidParam, "请提供 file_id 和可选知识标题")
		return
	}
	item, created, err := h.Service.Add(c.Request.Context(), userID, req.FileID, req.Title)
	if err != nil {
		h.fail(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusAccepted
	}
	response.SuccessWithStatus(c, status, item)
}

// List godoc
// @Summary 分页查询知识条目
// @Tags knowledge
// @Produce json
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页数量，默认20，最大100"
// @Param q query string false "标题包含匹配"
// @Param status query string false "pending/running/ready/failed"
// @Param vector query string false "pending/running/ready/partial"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Security ApiKeyAuth
// @Router /knowledge [get]
func (h *KnowledgeHandler) List(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		h.fail(c, service.ErrKnowledgeInvalid)
		return
	}
	size, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil {
		h.fail(c, service.ErrKnowledgeInvalid)
		return
	}
	items, total, err := h.Service.ListFiltered(c.Request.Context(), userID, page, size, c.Query("q"), model.KnowledgeStatus(c.Query("status")), model.VectorState(c.Query("vector")))
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": size})
}

// Chunks godoc
// @Summary 按序查询知识分块与向量状态
// @Tags knowledge
// @Produce json
// @Param id path int true "知识ID"
// @Success 200 {object} response.Response
// @Security ApiKeyAuth
// @Router /knowledge/{id}/chunks [get]
func (h *KnowledgeHandler) Chunks(c *gin.Context) {
	user, ok := knowledgeUser(c)
	if !ok {
		return
	}
	id, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	chunks, err := h.Service.Chunks(c.Request.Context(), user, id)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, gin.H{"items": chunks, "total": len(chunks)})
}

// Get godoc
// @Summary 知识详情及入库状态
// @Tags knowledge
// @Produce json
// @Param id path int true "知识ID"
// @Success 200 {object} response.Response{data=model.KnowledgeItem}
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Security ApiKeyAuth
// @Router /knowledge/{id} [get]
func (h *KnowledgeHandler) Get(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	id, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	item, err := h.Service.Get(c.Request.Context(), userID, id)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, item)
}

// ByFile godoc
// @Summary 查询文件的知识库状态
// @Description 未加入或无权访问返回404，文件表不新增知识状态字段。
// @Tags knowledge
// @Produce json
// @Param file_id path int true "文件ID"
// @Success 200 {object} response.Response{data=model.KnowledgeItem}
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Security ApiKeyAuth
// @Router /knowledge/by-file/{file_id} [get]
func (h *KnowledgeHandler) ByFile(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	id, ok := knowledgeID(c, c.Param("file_id"))
	if !ok {
		return
	}
	item, err := h.Service.ByFile(c.Request.Context(), userID, id)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, item)
}

// Retry godoc
// @Summary 重试失败的知识导入
// @Tags knowledge
// @Produce json
// @Param id path int true "知识ID"
// @Success 202 {object} response.Response{data=model.KnowledgeItem}
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 409 {object} response.Response
// @Security ApiKeyAuth
// @Router /knowledge/{id}/retry [post]
func (h *KnowledgeHandler) Retry(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	id, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	item, err := h.Service.Retry(c.Request.Context(), userID, id)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.SuccessWithStatus(c, http.StatusAccepted, item)
}
