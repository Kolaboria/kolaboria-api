package health

import "github.com/gin-gonic/gin"

func RegisterHealthRoutes(r *gin.Engine, handler *HealthHandler) {
	r.GET("/health", handler.Check)
}
