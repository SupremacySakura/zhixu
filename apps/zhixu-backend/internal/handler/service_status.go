package handler

import (
	"zhixu-backend/internal/response"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
)

type ServiceStatusHandler struct{ Service *service.ServiceStatusService }

// Get reports deployment service availability without invoking a model.
func (h *ServiceStatusHandler) Get(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	view, err := h.Service.Get(c.Request.Context())
	if err != nil {
		// The caller has left; do not write a spurious 500 or cancel other waiters.
		return
	}
	response.Success(c, view)
}
