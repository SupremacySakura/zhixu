package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRagSearchRequiresCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &ChatHandler{Service: &service.ChatService{}}
	r.POST("/internal/tools/v1/search", h.RagSearch)
	for _, token := range []string{"", "unknown"} {
		req := httptest.NewRequest("POST", "/internal/tools/v1/search", strings.NewReader(`{"query":"回滚"}`))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, 404, w.Code)
	}
}

func TestRagSearchRejectsUnknownMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &service.ChatService{}
	token, revoke, err := s.Register(&service.ChatCapability{
		Context: context.Background(), RagQuery: &service.RagQuery{UserID: 7},
	})
	require.NoError(t, err)
	defer revoke()
	r := gin.New()
	h := &ChatHandler{Service: s}
	r.POST("/internal/tools/v1/search", h.RagSearch)
	req := httptest.NewRequest("POST", "/internal/tools/v1/search", strings.NewReader(`{"query":"回滚","mode":"magic"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 400, w.Code)
	require.Contains(t, w.Body.String(), "检索模式无效")
}

func TestRagSearchClaimsCapabilityAcrossTurn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &service.ChatService{}
	token, revoke, err := s.Register(&service.ChatCapability{
		Context: context.Background(), RagQuery: &service.RagQuery{UserID: 7},
	})
	require.NoError(t, err)
	defer revoke()
	// A retrieval token survives repeated claims within one turn, unlike model tokens.
	require.NotNil(t, s.ClaimRagCapability(token))
	require.NotNil(t, s.ClaimRagCapability(token))
}
