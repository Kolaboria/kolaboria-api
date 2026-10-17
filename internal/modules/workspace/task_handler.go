package workspace

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

// ListWorkspaceTasks returns a page of project tasks with viewer-specific permissions.
// @Summary List Workspace tasks
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param page query int false "Page number"
// @Param limit query int false "Page size, maximum 50"
// @Param status query string false "Task status"
// @Param q query string false "Search task title and description"
// @Param mine query bool false "Only tasks assigned to the current member"
// @Param assignee_member_id query string false "Filter by assignee membership"
// @Success 200 {object} response.SuccessResponse{data=[]TaskListItemResponse,meta=project.ProjectPaginationMeta}
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/tasks [get]
func (handler *Handler) ListWorkspaceTasks(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	page, limit, err := parseWorkspacePagination(c.Query("page"), c.Query("limit"))
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	query := TaskListQuery{Page: page, Limit: limit, Query: c.Query("q")}
	if value := c.Query("status"); value != "" {
		status := TaskStatus(value)
		query.Status = &status
	}
	if value := c.Query("mine"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			writeWorkspaceTaskError(c, sharedErrors.ErrBadRequest)
			return
		}
		query.Mine = parsed
	}
	if value := c.Query("assignee_member_id"); value != "" {
		memberID, err := uuid.Parse(value)
		if err != nil || memberID == uuid.Nil {
			writeWorkspaceTaskError(c, sharedErrors.ErrBadRequest)
			return
		}
		query.AssigneeMemberID = &memberID
	}
	items, meta, err := handler.tasks.List(c.Request.Context(), actorID, c.Param("projectSlug"), query)
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	response.SuccessWithMeta(c, http.StatusOK, "Workspace tasks retrieved", items, meta)
}

// CreateWorkspaceTask creates a task and records its first revision and activity event.
// @Summary Create Workspace task
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param Idempotency-Key header string true "Unique retry key"
// @Param request body CreateTaskRequest true "Task fields"
// @Success 201 {object} response.SuccessResponse{data=TaskResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/tasks [post]
func (handler *Handler) CreateWorkspaceTask(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	var request CreateTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeWorkspaceTaskError(c, sharedErrors.ErrBadRequest)
		return
	}
	item, replayed, err := handler.tasks.Create(c.Request.Context(), actorID, c.Param("projectSlug"), c.GetHeader("Idempotency-Key"), request)
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	response.Success(c, status, "Workspace task created", item)
}

// GetWorkspaceTask returns the current task details and viewer-specific permissions.
// @Summary Get Workspace task
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param taskID path string true "Task ID"
// @Success 200 {object} response.SuccessResponse{data=TaskResponse}
// @Failure 404 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/tasks/{taskID} [get]
func (handler *Handler) GetWorkspaceTask(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	taskID, err := parseTaskID(c.Param("taskID"))
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	item, err := handler.tasks.Get(c.Request.Context(), actorID, c.Param("projectSlug"), taskID)
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	response.OK(c, "Workspace task retrieved", item)
}

// UpdateWorkspaceTask updates task content, metadata, or status with optimistic concurrency.
// @Summary Update Workspace task
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param taskID path string true "Task ID"
// @Param request body UpdateTaskRequest true "Task changes"
// @Success 200 {object} response.SuccessResponse{data=TaskResponse}
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/tasks/{taskID} [patch]
func (handler *Handler) UpdateWorkspaceTask(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	taskID, err := parseTaskID(c.Param("taskID"))
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	var request UpdateTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeWorkspaceTaskError(c, sharedErrors.ErrBadRequest)
		return
	}
	item, err := handler.tasks.Update(c.Request.Context(), actorID, c.Param("projectSlug"), taskID, request)
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	response.OK(c, "Workspace task updated", item)
}

// TransitionWorkspaceTask moves a task through its review lifecycle.
// @Summary Change Workspace task status
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param taskID path string true "Task ID"
// @Param request body TransitionTaskRequest true "Status transition"
// @Success 200 {object} response.SuccessResponse{data=TaskResponse}
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/tasks/{taskID}/transitions [post]
func (handler *Handler) TransitionWorkspaceTask(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	taskID, err := parseTaskID(c.Param("taskID"))
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	var request TransitionTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeWorkspaceTaskError(c, sharedErrors.ErrBadRequest)
		return
	}
	item, err := handler.tasks.Transition(c.Request.Context(), actorID, c.Param("projectSlug"), taskID, request)
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	response.OK(c, "Workspace task status updated", item)
}

// DeleteWorkspaceTask soft-deletes an active task while retaining its evidence history.
// @Summary Delete Workspace task
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param taskID path string true "Task ID"
// @Param version query int true "Current task version"
// @Success 200 {object} response.SuccessResponse{data=TaskTombstoneResponse}
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/tasks/{taskID} [delete]
func (handler *Handler) DeleteWorkspaceTask(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	taskID, err := parseTaskID(c.Param("taskID"))
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	version, err := strconv.ParseInt(c.Query("version"), 10, 64)
	if err != nil || version < 1 {
		writeWorkspaceTaskError(c, sharedErrors.ErrBadRequest)
		return
	}
	item, err := handler.tasks.Delete(c.Request.Context(), actorID, c.Param("projectSlug"), taskID, version)
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	response.OK(c, "Workspace task deleted", item)
}

// ListWorkspaceTaskComments returns task comments in creation order.
// @Summary List Workspace task comments
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param taskID path string true "Task ID"
// @Param page query int false "Page number"
// @Param limit query int false "Page size, maximum 50"
// @Success 200 {object} response.SuccessResponse{data=[]TaskCommentResponse,meta=project.ProjectPaginationMeta}
// @Failure 404 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/tasks/{taskID}/comments [get]
func (handler *Handler) ListWorkspaceTaskComments(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	taskID, err := parseTaskID(c.Param("taskID"))
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	page, limit, err := parseWorkspacePagination(c.Query("page"), c.Query("limit"))
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	items, meta, err := handler.tasks.ListComments(c.Request.Context(), actorID, c.Param("projectSlug"), taskID, page, limit)
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	response.SuccessWithMeta(c, http.StatusOK, "Workspace task comments retrieved", items, meta)
}

// CreateWorkspaceTaskComment adds an immutable task comment without changing its review decision.
// @Summary Create Workspace task comment
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param taskID path string true "Task ID"
// @Param Idempotency-Key header string true "Unique retry key"
// @Param request body CreateTaskCommentRequest true "Comment text"
// @Success 201 {object} response.SuccessResponse{data=TaskCommentResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/tasks/{taskID}/comments [post]
func (handler *Handler) CreateWorkspaceTaskComment(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	taskID, err := parseTaskID(c.Param("taskID"))
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	var request CreateTaskCommentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeWorkspaceTaskError(c, sharedErrors.ErrBadRequest)
		return
	}
	item, replayed, err := handler.tasks.CreateComment(c.Request.Context(), actorID, c.Param("projectSlug"), taskID, c.GetHeader("Idempotency-Key"), request)
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	response.Success(c, status, "Workspace task comment created", item)
}

// UpdateWorkspaceTaskAccess grants or revokes contributor task creation and deletion.
// @Summary Update Workspace task access
// @Tags Workspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param memberID path string true "Project membership ID"
// @Param request body UpdateTaskAccessRequest true "Task access change"
// @Success 200 {object} response.SuccessResponse{data=TaskAccessResponse}
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/members/{memberID}/task-access [patch]
func (handler *Handler) UpdateWorkspaceTaskAccess(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	memberID, err := parseTaskID(c.Param("memberID"))
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	var request UpdateTaskAccessRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeWorkspaceTaskError(c, sharedErrors.ErrBadRequest)
		return
	}
	item, err := handler.tasks.UpdateTaskAccess(c.Request.Context(), actorID, c.Param("projectSlug"), memberID, request)
	if err != nil {
		writeWorkspaceTaskError(c, err)
		return
	}
	response.OK(c, "Workspace task access updated", item)
}

func parseTaskID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, sharedErrors.ErrBadRequest
	}
	return id, nil
}

func writeWorkspaceTaskError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sharedErrors.ErrUnauthorized):
		response.ErrorWithException(c, http.StatusUnauthorized, "Authentication is required", err)
	case errors.Is(err, sharedErrors.ErrForbidden):
		response.ErrorWithException(c, http.StatusForbidden, "You are not allowed to change this workspace task", err)
	case errors.Is(err, sharedErrors.ErrNotFound):
		response.ErrorWithException(c, http.StatusNotFound, "Workspace task or project not found", err)
	case errors.Is(err, sharedErrors.ErrBadRequest):
		response.ErrorWithException(c, http.StatusBadRequest, "Workspace task request is invalid", err)
	case errors.Is(err, sharedErrors.ErrConflict):
		response.ErrorWithException(c, http.StatusConflict, "Workspace task conflicts with the current project state", err)
	default:
		response.ErrorWithException(c, http.StatusInternalServerError, "Unable to process workspace task request", err)
	}
}
