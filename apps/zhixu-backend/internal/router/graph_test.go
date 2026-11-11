package router

import (
	"testing"

	"zhixu-backend/internal/repository"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 静态 /knowledge/graph 与既有 /knowledge/:id 必须能共存，且各自注册成功。
func TestGraphRouterCoexistsWithKnowledgeIDRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NotPanics(t, func() {
		RegisterGraphRouter(r, service.NewGraphService(repository.NewGraphRepository(nil)))
		RegisterKnowledgeRouter(r, service.NewKnowledgeService(repository.NewKnowledgeRepository(nil), ""))
	})
	routes := map[string]bool{}
	for _, route := range r.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	require.True(t, routes["GET /knowledge/graph"])
	require.True(t, routes["GET /knowledge/:id"])
}
