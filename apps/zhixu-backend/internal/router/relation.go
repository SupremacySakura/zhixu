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
	knowledge.POST("/:id/relations/regenerate", h.Regenerate)
	relations := r.Group("/relations", middleware.AuthMiddleware())
	relations.PUT("/:id", h.Update)
	relations.DELETE("/:id", h.Delete)
	relations.POST("/:id/confirm", h.Confirm)
	relations.POST("/:id/reject", h.Reject)
}
