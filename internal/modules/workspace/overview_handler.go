package workspace

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

func (handler *Handler) WithOverview(service *OverviewService, admission *OverviewAdmission) *Handler {
	handler.overview = service
	handler.overviewAdmission = admission
	return handler
}

// GetWorkspaceOverview returns one bounded snapshot for the authenticated project member.
// @Summary Get Workspace overview
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Success 200 {object} response.SuccessResponse{data=WorkspaceOverviewResponse}
// @Failure 401,404,503,504 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/overview [get]
func (handler *Handler) GetWorkspaceOverview(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	result, err := handler.overview.Get(c.Request.Context(), actorID, c.Param("projectSlug"))
	if err != nil {
		writeWorkspaceOverviewError(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, http.StatusOK, "Workspace overview retrieved", result)
}

func writeWorkspaceOverviewError(c *gin.Context, err error) {
	status, message := http.StatusInternalServerError, "Unable to load workspace overview"
	switch {
	case errors.Is(err, sharedErrors.ErrUnauthorized):
		status, message = http.StatusUnauthorized, "Authentication is required"
	case errors.Is(err, sharedErrors.ErrNotFound):
		status, message = http.StatusNotFound, "Project not found"
	case errors.Is(err, sharedErrors.ErrBadRequest):
		status, message = http.StatusBadRequest, "Workspace overview request is invalid"
	case errors.Is(err, ErrOverviewOverloaded):
		status, message = http.StatusServiceUnavailable, "Workspace overview is busy; try again shortly"
		c.Header("Retry-After", "1")
	case errors.Is(err, context.DeadlineExceeded):
		status, message = http.StatusGatewayTimeout, "Workspace overview took too long to load"
	}
	response.ErrorWithException(c, status, message, err)
}
