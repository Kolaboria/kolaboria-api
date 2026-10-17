package workspace

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

func NewHandlerWithActivities(service *Service, team *TeamService, tasks *TaskService, discussions *DiscussionService, activities *ActivityService) *Handler {
	handler := NewHandlerWithTasks(service, team, tasks, discussions)
	handler.activities = activities
	return handler
}

func writeWorkspaceActivityError(c *gin.Context, err error) {
	status, message := http.StatusInternalServerError, "Unable to load workspace activities"
	switch {
	case errors.Is(err, sharedErrors.ErrUnauthorized):
		status, message = http.StatusUnauthorized, "Authentication is required"
	case errors.Is(err, sharedErrors.ErrNotFound):
		status, message = http.StatusNotFound, "Project not found"
	case errors.Is(err, sharedErrors.ErrBadRequest):
		status, message = http.StatusBadRequest, "Workspace activity request is invalid"
	}
	response.ErrorWithException(c, status, message, err)
}

// ListWorkspaceActivities returns a page of member-visible workspace facts.
// @Summary List Workspace activities
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param page query int false "Page number"
// @Param limit query int false "Page size, maximum 50"
// @Success 200 {object} response.SuccessResponse{data=[]ActivityResponse,meta=project.ProjectPaginationMeta}
// @Failure 400,401,404,500 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/activities [get]
func (handler *Handler) ListWorkspaceActivities(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	page, limit, err := parseWorkspacePagination(c.Query("page"), c.Query("limit"))
	if err != nil {
		writeWorkspaceActivityError(c, err)
		return
	}
	items, meta, err := handler.activities.List(c.Request.Context(), actorID, c.Param("projectSlug"), page, limit)
	if err != nil {
		writeWorkspaceActivityError(c, err)
		return
	}
	response.SuccessWithMeta(c, http.StatusOK, "Workspace activities retrieved", items, meta)
}
