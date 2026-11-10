package router

import (
	"zhixu-backend/internal/handler"
	"zhixu-backend/internal/middleware"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
)

func RegisterKnowledgeRouter(r *gin.Engine, svc *service.KnowledgeService) {
	h := &handler.KnowledgeHandler{Service: svc}
	group := r.Group("/knowledge", middleware.AuthMiddleware())
	group.POST("", h.Add)
	group.GET("", h.List)
	group.GET("/by-file/:file_id", h.ByFile)
	group.GET("/:id", h.Get)
	group.PUT("/:id", h.Update)
	group.POST("/:id/favorite", h.Favorite)
	group.POST("/:id/archive", h.Archive)
	group.POST("/:id/unarchive", h.Unarchive)
	group.GET("/:id/versions", h.Versions)
	group.GET("/:id/versions/:version", h.VersionDetail)
	group.POST("/:id/versions/:version/restore", h.RestoreVersion)
	group.GET("/:id/chunks", h.Chunks)
	group.DELETE("/:id", h.Delete)
	group.POST("/:id/retry", h.Retry)
	group.POST("/:id/vectorize/retry", h.RetryVectorize)
}
