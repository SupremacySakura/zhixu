package router

import (
	"zhixu-backend/internal/handler"
	"zhixu-backend/internal/middleware"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterGraphRouter 注册局部图谱接口；静态 /graph 必须先于 GET /knowledge/:id 注册，
// 避免中心知识参数被 :id 捕获（gin 同时支持静态段优先，这里显式保证）。
func RegisterGraphRouter(r *gin.Engine, svc *service.GraphService) {
	group := r.Group("/knowledge", middleware.AuthMiddleware())
	group.GET("/graph", (&handler.GraphHandler{Service: svc}).Get)
}
