package router

import (
	"zhixu-backend/internal/handler"
	"zhixu-backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRelationRouter 挂载知识关系接口。
func RegisterRelationRouter(r *gin.Engine, h *handler.RelationHandler) {
	knowledge := r.Group("/knowledge", middleware.AuthMiddleware())
	knowledge.GET("/:id/relations", h.List)
	knowledge.POST("/:id/relations", h.Create)
	relations := r.Group("/relations", middleware.AuthMiddleware())
	relations.DELETE("/:id", h.Delete)
}
