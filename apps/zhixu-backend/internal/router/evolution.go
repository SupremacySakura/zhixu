package router

import (
	"zhixu-backend/internal/handler"
	"zhixu-backend/internal/middleware"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterEvolutionRouter 演化提醒接口。
// 重跑检查的路径挂在 /knowledge 下（必须带知识 ID），与提醒列表分开注册但共用同一个 handler。
func RegisterEvolutionRouter(r *gin.Engine, svc *service.EvolutionService) {
	h := &handler.EvolutionHandler{Service: svc}
	group := r.Group("/evolution", middleware.AuthMiddleware())
	group.GET("/alerts", h.List)
	group.GET("/alerts/summary", h.Summary)
	group.POST("/alerts/:id/resolve", h.Resolve)

	knowledge := r.Group("/knowledge", middleware.AuthMiddleware())
	knowledge.POST("/:id/evolve/regenerate", h.Regenerate)
}
