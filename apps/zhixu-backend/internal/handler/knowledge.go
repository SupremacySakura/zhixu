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

// AddKnowledgeRequest 二选一：file_id 从文件导入，或 title+content 手动创建。
type AddKnowledgeRequest struct {
	FileID  uint   `json:"file_id"`
	Title   string `json:"title"`
	Content string `json:"content"`
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
		response.FailWithStatus(c, http.StatusConflict, response.CodeConflict, "当前状态不支持该操作，请刷新后重试")
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

func knowledgeVersion(c *gin.Context, raw string) (uint, bool) {
	version, err := strconv.ParseUint(raw, 10, strconv.IntSize)
	if err != nil || version == 0 {
		response.FailWithStatus(c, 400, response.CodeInvalidParam, "版本参数无效")
		return 0, false
	}
	return uint(version), true
}

// Add godoc
// @Summary 创建知识：file_id 从文件导入，或 title+content 手动创建（二选一）
// @Description 文件导入首次加入返回202，重复请求返回原知识条目；手动创建始终返回202。
// @Tags knowledge
// @Accept json
// @Produce json
// @Param data body AddKnowledgeRequest true "file_id 与可选标题，或 title+content"
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
	// 手动创建正文上限 2M 字符，请求体放宽到 4MB；文件导入只用得到 file_id 与标题。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<20)
	if c.ShouldBindJSON(&req) != nil {
		response.FailWithStatus(c, 400, response.CodeInvalidParam, "请求格式不正确或超出大小限制")
		return
	}
	if req.FileID > 0 {
		if req.Content != "" {
			response.FailWithStatus(c, 400, response.CodeInvalidParam, "file_id 与 content 只能二选一")
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
		return
	}
	// 手动创建：标题与正文的完整校验在服务层。
	item, err := h.Service.CreateManual(c.Request.Context(), userID, req.Title, req.Content)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.SuccessWithStatus(c, http.StatusAccepted, item)
}

// UpdateKnowledgeRequest 编辑知识正文；base_version 为乐观锁版本号。
type UpdateKnowledgeRequest struct {
	Title       string `json:"title"`
	Content     string `json:"content"`
	BaseVersion uint   `json:"base_version"`
}

// Update godoc
// @Summary 编辑知识正文并生成版本快照
// @Description 仅 ready 状态可编辑；base_version 与当前版本不一致返回409，正文更新后重新分块并向量化。
// @Tags knowledge
// @Accept json
// @Produce json
// @Param id path int true "知识ID"
// @Param data body UpdateKnowledgeRequest true "新标题、正文与当前版本号"
// @Success 200 {object} response.Response{data=model.KnowledgeItem}
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 409 {object} response.Response
// @Failure 500 {object} response.Response
// @Security ApiKeyAuth
// @Router /knowledge/{id} [put]
func (h *KnowledgeHandler) Update(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	id, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	var req UpdateKnowledgeRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<20)
	if c.ShouldBindJSON(&req) != nil {
		response.FailWithStatus(c, 400, response.CodeInvalidParam, "请求格式不正确或超出大小限制")
		return
	}
	item, err := h.Service.Update(c.Request.Context(), userID, id, req.Title, req.Content, req.BaseVersion)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, item)
}

// Versions godoc
// @Summary 知识版本历史
// @Description 当前版本置顶并由条目实时合成，其余为不可变快照；列表不含正文。
// @Tags knowledge
// @Produce json
// @Param id path int true "知识ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Security ApiKeyAuth
// @Router /knowledge/{id}/versions [get]
func (h *KnowledgeHandler) Versions(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	id, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	versions, err := h.Service.Versions(c.Request.Context(), userID, id)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, gin.H{"items": versions, "total": len(versions)})
}

// VersionDetail godoc
// @Summary 查看指定版本正文
// @Description 版本号等于当前版本时返回条目实时内容。
// @Tags knowledge
// @Produce json
// @Param id path int true "知识ID"
// @Param version path int true "版本号"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Security ApiKeyAuth
// @Router /knowledge/{id}/versions/{version} [get]
func (h *KnowledgeHandler) VersionDetail(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	id, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	version, ok := knowledgeVersion(c, c.Param("version"))
	if !ok {
		return
	}
	detail, err := h.Service.VersionDetail(c.Request.Context(), userID, id, version)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, detail)
}

// RestoreVersion godoc
// @Summary 恢复历史版本
// @Description 以目标版本内容生成新版本并重新向量化，当前正文先写入快照，历史记录不被篡改。
// @Tags knowledge
// @Produce json
// @Param id path int true "知识ID"
// @Param version path int true "版本号"
// @Success 200 {object} response.Response{data=model.KnowledgeItem}
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 409 {object} response.Response
// @Security ApiKeyAuth
// @Router /knowledge/{id}/versions/{version}/restore [post]
func (h *KnowledgeHandler) RestoreVersion(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	id, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	version, ok := knowledgeVersion(c, c.Param("version"))
	if !ok {
		return
	}
	item, err := h.Service.Restore(c.Request.Context(), userID, id, version)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.Success(c, item)
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

// RetryVectorize godoc
// @Summary 重试部分失败的向量化任务
// @Description 仅重试失败分块，已成功分块不重复计费。返回202仅代表已重新排队。
// @Tags knowledge
// @Produce json
// @Param id path int true "知识ID"
// @Success 202 {object} response.Response{data=model.KnowledgeItem}
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 409 {object} response.Response
// @Security ApiKeyAuth
// @Router /knowledge/{id}/vectorize/retry [post]
func (h *KnowledgeHandler) RetryVectorize(c *gin.Context) {
	userID, ok := knowledgeUser(c)
	if !ok {
		return
	}
	id, ok := knowledgeID(c, c.Param("id"))
	if !ok {
		return
	}
	item, err := h.Service.RetryVectorize(c.Request.Context(), userID, id)
	if err != nil {
		h.fail(c, err)
		return
	}
	response.SuccessWithStatus(c, http.StatusAccepted, item)
}
