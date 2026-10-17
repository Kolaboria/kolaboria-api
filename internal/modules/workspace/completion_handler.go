package workspace

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

func (handler *Handler) WithCompletion(service *CompletionService) *Handler {
	handler.completion = service
	return handler
}

// CompleteWorkspaceProject records the owner's review and completes the project.
// @Summary Complete Workspace project
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param request body CompleteProjectRequest true "Versions and owner review"
// @Success 200 {object} response.SuccessResponse{data=ProjectCompletionResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/completion [post]
func (handler *Handler) CompleteWorkspaceProject(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	var request CompleteProjectRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeWorkspaceCompletionError(c, fmt.Errorf("%w: %v", sharedErrors.ErrBadRequest, err))
		return
	}
	result, err := handler.completion.Complete(c.Request.Context(), actorID, c.Param("projectSlug"), request)
	if err != nil {
		writeWorkspaceCompletionError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Workspace project completed", result)
}

func writeWorkspaceCompletionError(c *gin.Context, err error) {
	status, message := http.StatusInternalServerError, "Unable to complete workspace project"
	switch {
	case errors.Is(err, sharedErrors.ErrUnauthorized):
		status, message = http.StatusUnauthorized, "Authentication is required"
	case errors.Is(err, sharedErrors.ErrForbidden):
		status, message = http.StatusForbidden, "Only the project owner can complete this project"
	case errors.Is(err, sharedErrors.ErrNotFound):
		status, message = http.StatusNotFound, "Project not found"
	case errors.Is(err, sharedErrors.ErrBadRequest):
		status, message = http.StatusBadRequest, "Project completion review is invalid"
	case errors.Is(err, sharedErrors.ErrConflict):
		status, message = http.StatusConflict, "Project or task state changed; review the latest closure summary"
	}
	response.ErrorWithException(c, status, message, err)
}
