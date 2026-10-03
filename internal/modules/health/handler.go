package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Check handles the health check request.
// @Summary      Health Check
// @Description  Check if the application is running and healthy
// @Tags         health
// @Produce      json
// @Success      200  {object}  response.SuccessResponse
// @Router       /health [get]
func (h *HealthHandler) Check(c *gin.Context) {
	response.Success(c, http.StatusOK, "Application is running", map[string]string{
		"status": "UP",
	})
}
