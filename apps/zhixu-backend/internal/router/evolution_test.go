package router

import (
	"testing"

	"zhixu-backend/internal/repository"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 演化提醒路由与既有 /knowledge/:id 系列必须能共存：
// 重跑检查挂在 /knowledge/:id/evolve/regenerate 下，与 /knowledge/graph 同属静态前缀竞争区。
func TestEvolutionRouterCoexistsWithKnowledgeRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NotPanics(t, func() {
		RegisterEvolutionRouter(r, &service.EvolutionService{})
		RegisterKnowledgeRouter(r, service.NewKnowledgeService(repository.NewKnowledgeRepository(nil), ""))
		RegisterGraphRouter(r, service.NewGraphService(repository.NewGraphRepository(nil)))
	})
	routes := map[string]bool{}
	for _, route := range r.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /evolution/alerts",
		"GET /evolution/alerts/summary",
		"POST /evolution/alerts/:id/resolve",
		"POST /knowledge/:id/evolve/regenerate",
		"GET /knowledge/:id",
		"GET /knowledge/graph",
	} {
		require.True(t, routes[want], "缺少路由 "+want)
	}
}
