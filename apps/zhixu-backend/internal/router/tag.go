package router

import (
	"zhixu-backend/internal/handler"
	"zhixu-backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterTagRouter 注册标签路由（均需登录）。
func RegisterTagRouter(r *gin.Engine, h *handler.TagHandler) {
	group := r.Group("/tags", middleware.AuthMiddleware())
	group.GET("", h.List)
	group.POST("", h.Create)
	group.PUT("/:id", h.Rename)
	group.DELETE("/:id", h.Delete)

	knowledge := r.Group("/knowledge", middleware.AuthMiddleware())
	knowledge.GET("/:id/tags", h.KnowledgeTags)
	knowledge.PUT("/:id/tags", h.ReplaceKnowledgeTags)
}
