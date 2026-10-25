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
	r.POST("/internal/rag/v1/search", h.RagSearch)
	for _, token := range []string{"", "unknown"} {
		req := httptest.NewRequest("POST", "/internal/rag/v1/search", strings.NewReader(`{"query":"回滚"}`))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, 404, w.Code)
	}
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
	// Model tokens stay single-use.
	modelToken, modelRevoke, err := s.Register(&service.ChatCapability{Context: context.Background()})
	require.NoError(t, err)
	defer modelRevoke()
	require.NotNil(t, s.ClaimCapability(modelToken))
	require.Nil(t, s.ClaimCapability(modelToken))
}
