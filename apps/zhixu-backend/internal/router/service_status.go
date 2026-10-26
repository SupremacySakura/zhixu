package router

import (
	"zhixu-backend/internal/handler"
	"zhixu-backend/internal/middleware"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
)

func RegisterServiceStatusRouter(r *gin.Engine, svc *service.ServiceStatusService) {
	h := &handler.ServiceStatusHandler{Service: svc}
	r.GET("/settings/services/status", middleware.AuthMiddleware(), h.Get)
}
