package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
)

type ragSearchRequest struct {
	Query string `json:"query"`
	// Mode selects the retrieval legs: hybrid (default), semantic or keyword.
	// It is part of the internal contract so the three-mode experiment can run
	// through the same gateway the chat tool uses.
	Mode string `json:"mode"`
	// TopK is accepted for AI-service contract compatibility but ignored:
	// retrieval breadth and the similarity threshold come from the user's settings.
	TopK int `json:"topK"`
}

// RagSearch is the internal retrieval gateway called only by the AI service with a
// capability issued by the chat handler. The token carries the user id, so a token
// from one tenant can never read another tenant's chunks.
func (h *ChatHandler) RagSearch(c *gin.Context) {
	cap := h.Service.ClaimRagCapability(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	if cap == nil || cap.RagQuery == nil {
		c.Status(404)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8*1024)
	var input ragSearchRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		c.Status(400)
		return
	}
	if !service.ValidRagMode(input.Mode) {
		c.JSON(400, gin.H{"error": "检索模式无效"})
		return
	}
	result, err := h.Rag.Search(cap.Context, service.RagQuery{
		UserID:         cap.RagQuery.UserID,
		Query:          strings.TrimSpace(input.Query),
		Mode:           input.Mode,
		ConversationID: cap.RagQuery.ConversationID,
	})
	if err != nil {
		// Never echo embedding, database, or Qdrant failures back to the AI service.
		c.JSON(502, gin.H{"error": "检索服务暂时不可用"})
		return
	}
	c.JSON(200, result)
}
