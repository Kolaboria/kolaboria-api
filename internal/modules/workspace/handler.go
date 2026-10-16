package workspace

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

type Handler struct {
	service     *Service
	discussions *DiscussionService
}

func NewHandler(service *Service, discussions ...*DiscussionService) *Handler {
	handler := &Handler{service: service}
	if len(discussions) > 0 {
		handler.discussions = discussions[0]
	}
	return handler
}

// GetProjectInfo returns the authenticated viewer's project Workspace Info.
// @Summary Get Workspace project info
// @Tags Workspace
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Success 200 {object} response.SuccessResponse{data=ProjectInfoResponse}
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/info [get]
func (handler *Handler) GetProjectInfo(c *gin.Context) {
	actorID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil || actorID == uuid.Nil {
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
		return
	}
	result, err := handler.service.GetProjectInfo(c.Request.Context(), actorID, c.Param("projectSlug"))
	if err != nil {
		writeWorkspaceError(c, err)
		return
	}
	response.OK(c, "Workspace project info retrieved", result)
}

func writeWorkspaceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sharedErrors.ErrUnauthorized):
		response.ErrorWithException(c, http.StatusUnauthorized, "Authentication is required", err)
	case errors.Is(err, sharedErrors.ErrForbidden):
		response.ErrorWithException(c, http.StatusForbidden, "You are not allowed to view this workspace", err)
	case errors.Is(err, sharedErrors.ErrNotFound):
		response.ErrorWithException(c, http.StatusNotFound, "Project not found", err)
	case errors.Is(err, sharedErrors.ErrBadRequest):
		response.ErrorWithException(c, http.StatusBadRequest, "Workspace request is invalid", err)
	case errors.Is(err, sharedErrors.ErrConflict):
		response.ErrorWithException(c, http.StatusConflict, "Workspace resource conflicts with the current project state", err)
	default:
		response.ErrorWithException(c, http.StatusInternalServerError, "Unable to load workspace project info", err)
	}
}

func writeWorkspaceDiscussionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sharedErrors.ErrUnauthorized):
		response.ErrorWithException(c, http.StatusUnauthorized, "Authentication is required", err)
	case errors.Is(err, sharedErrors.ErrForbidden):
		response.ErrorWithException(c, http.StatusForbidden, "You are not allowed to change this discussion", err)
	case errors.Is(err, sharedErrors.ErrNotFound):
		response.ErrorWithException(c, http.StatusNotFound, "Project or discussion not found", err)
	case errors.Is(err, sharedErrors.ErrBadRequest):
		response.ErrorWithException(c, http.StatusBadRequest, "Workspace discussion request is invalid", err)
	case errors.Is(err, sharedErrors.ErrConflict):
		response.ErrorWithException(c, http.StatusConflict, "Workspace discussion conflicts with the current project state", err)
	default:
		response.ErrorWithException(c, http.StatusInternalServerError, "Unable to process workspace discussion", err)
	}
}

// ListDiscussions returns a stable page of active discussions in a project.
// @Summary List Workspace discussions
// @Tags Workspace Discussions
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param page query int false "Page number"
// @Param limit query int false "Page size, maximum 50"
// @Success 200 {object} response.SuccessResponse{data=[]DiscussionListItemResponse,meta=project.ProjectPaginationMeta}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/discussions [get]
func (handler *Handler) ListDiscussions(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	page, limit, err := parseWorkspacePagination(c.Query("page"), c.Query("limit"))
	if err != nil {
		writeWorkspaceDiscussionError(c, err)
		return
	}
	items, meta, err := handler.discussions.List(c.Request.Context(), actorID, c.Param("projectSlug"), page, limit)
	if err != nil {
		writeWorkspaceDiscussionError(c, err)
		return
	}
	response.SuccessWithMeta(c, http.StatusOK, "Workspace discussions retrieved", items, meta)
}

// GetDiscussion returns one discussion scoped to the requested project.
// @Summary Get Workspace discussion
// @Tags Workspace Discussions
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param discussionID path string true "Discussion UUID"
// @Success 200 {object} response.SuccessResponse{data=DiscussionResponse}
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/discussions/{discussionID} [get]
func (handler *Handler) GetDiscussion(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	result, err := handler.discussions.Get(c.Request.Context(), actorID, c.Param("projectSlug"), c.Param("discussionID"))
	if err != nil {
		writeWorkspaceDiscussionError(c, err)
		return
	}
	response.OK(c, "Workspace discussion retrieved", result)
}

// CreateDiscussion creates a project discussion. Idempotency-Key is required.
// @Summary Create Workspace discussion
// @Tags Workspace Discussions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param Idempotency-Key header string true "Unique retry key"
// @Param request body CreateDiscussionRequest true "Discussion title and body"
// @Success 201 {object} response.SuccessResponse{data=DiscussionResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/discussions [post]
func (handler *Handler) CreateDiscussion(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	var request CreateDiscussionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeWorkspaceDiscussionError(c, fmt.Errorf("bind create discussion request: %w", err))
		return
	}
	result, created, err := handler.discussions.Create(c.Request.Context(), actorID, c.Param("projectSlug"), c.GetHeader("Idempotency-Key"), request)
	if err != nil {
		writeWorkspaceDiscussionError(c, err)
		return
	}
	status, message := http.StatusOK, "Workspace discussion already created"
	if created {
		status, message = http.StatusCreated, "Workspace discussion created"
	}
	response.Success(c, status, message, result)
}

// UpdateDiscussion edits a discussion authored by the current user.
// @Summary Update Workspace discussion
// @Tags Workspace Discussions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param discussionID path string true "Discussion UUID"
// @Param request body UpdateDiscussionRequest true "Discussion fields to update"
// @Success 200 {object} response.SuccessResponse{data=DiscussionResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/discussions/{discussionID} [patch]
func (handler *Handler) UpdateDiscussion(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	var request UpdateDiscussionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeWorkspaceDiscussionError(c, fmt.Errorf("bind update discussion request: %w", err))
		return
	}
	result, err := handler.discussions.Update(c.Request.Context(), actorID, c.Param("projectSlug"), c.Param("discussionID"), request)
	if err != nil {
		writeWorkspaceDiscussionError(c, err)
		return
	}
	response.OK(c, "Workspace discussion updated", result)
}

// DeleteDiscussion removes a discussion authored by the user or any discussion as project owner.
// @Summary Delete Workspace discussion
// @Tags Workspace Discussions
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param discussionID path string true "Discussion UUID"
// @Success 200 {object} response.SuccessResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/discussions/{discussionID} [delete]
func (handler *Handler) DeleteDiscussion(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	if err := handler.discussions.Delete(c.Request.Context(), actorID, c.Param("projectSlug"), c.Param("discussionID")); err != nil {
		writeWorkspaceDiscussionError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Workspace discussion deleted", nil)
}

// ListDiscussionReplies returns root replies and their direct follow-ups.
// @Summary List Workspace discussion replies
// @Tags Workspace Discussions
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param discussionID path string true "Discussion UUID"
// @Param page query int false "Page number"
// @Param limit query int false "Page size, maximum 50"
// @Success 200 {object} response.SuccessResponse{data=[]DiscussionReplyResponse,meta=project.ProjectPaginationMeta}
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/discussions/{discussionID}/replies [get]
func (handler *Handler) ListDiscussionReplies(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	page, limit, err := parseWorkspacePagination(c.Query("page"), c.Query("limit"))
	if err != nil {
		writeWorkspaceDiscussionError(c, err)
		return
	}
	items, meta, err := handler.discussions.ListReplies(c.Request.Context(), actorID, c.Param("projectSlug"), c.Param("discussionID"), page, limit)
	if err != nil {
		writeWorkspaceDiscussionError(c, err)
		return
	}
	response.SuccessWithMeta(c, http.StatusOK, "Workspace discussion replies retrieved", items, meta)
}

// CreateDiscussionReply creates a root reply or one direct follow-up. Idempotency-Key is required.
// @Summary Create Workspace discussion reply
// @Tags Workspace Discussions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param discussionID path string true "Discussion UUID"
// @Param Idempotency-Key header string true "Unique retry key"
// @Param request body CreateDiscussionReplyRequest true "Reply body and optional root parent"
// @Success 201 {object} response.SuccessResponse{data=DiscussionReplyResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/discussions/{discussionID}/replies [post]
func (handler *Handler) CreateDiscussionReply(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	var request CreateDiscussionReplyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeWorkspaceDiscussionError(c, fmt.Errorf("bind create discussion reply request: %w", err))
		return
	}
	result, created, err := handler.discussions.CreateReply(c.Request.Context(), actorID, c.Param("projectSlug"), c.Param("discussionID"), c.GetHeader("Idempotency-Key"), request)
	if err != nil {
		writeWorkspaceDiscussionError(c, err)
		return
	}
	status, message := http.StatusOK, "Workspace discussion reply already created"
	if created {
		status, message = http.StatusCreated, "Workspace discussion reply created"
	}
	response.Success(c, status, message, result)
}

// UpdateDiscussionReply edits a reply authored by the current user.
// @Summary Update Workspace discussion reply
// @Tags Workspace Discussions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param discussionID path string true "Discussion UUID"
// @Param replyID path string true "Reply UUID"
// @Param request body UpdateDiscussionReplyRequest true "Reply body"
// @Success 200 {object} response.SuccessResponse{data=DiscussionReplyResponse}
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/discussions/{discussionID}/replies/{replyID} [patch]
func (handler *Handler) UpdateDiscussionReply(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	var request UpdateDiscussionReplyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeWorkspaceDiscussionError(c, fmt.Errorf("bind update discussion reply request: %w", err))
		return
	}
	result, err := handler.discussions.UpdateReply(c.Request.Context(), actorID, c.Param("projectSlug"), c.Param("discussionID"), c.Param("replyID"), request)
	if err != nil {
		writeWorkspaceDiscussionError(c, err)
		return
	}
	response.OK(c, "Workspace discussion reply updated", result)
}

// DeleteDiscussionReply removes a reply authored by the user or any reply as project owner.
// @Summary Delete Workspace discussion reply
// @Tags Workspace Discussions
// @Produce json
// @Security BearerAuth
// @Param projectSlug path string true "Project slug"
// @Param discussionID path string true "Discussion UUID"
// @Param replyID path string true "Reply UUID"
// @Success 200 {object} response.SuccessResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /v1/workspaces/{projectSlug}/discussions/{discussionID}/replies/{replyID} [delete]
func (handler *Handler) DeleteDiscussionReply(c *gin.Context) {
	actorID, ok := workspaceActorID(c)
	if !ok {
		return
	}
	if err := handler.discussions.DeleteReply(c.Request.Context(), actorID, c.Param("projectSlug"), c.Param("discussionID"), c.Param("replyID")); err != nil {
		writeWorkspaceDiscussionError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Workspace discussion reply deleted", nil)
}

func workspaceActorID(c *gin.Context) (uuid.UUID, bool) {
	actorID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil || actorID == uuid.Nil {
		response.ErrorWithException(c, http.StatusUnauthorized, "Authentication is required", sharedErrors.ErrUnauthorized)
		return uuid.Nil, false
	}
	return actorID, true
}

func parseWorkspacePagination(pageValue, limitValue string) (int, int, error) {
	page, limit := 1, DiscussionDefaultPageSize
	if pageValue != "" {
		parsed, err := strconv.Atoi(pageValue)
		if err != nil || parsed < 1 {
			return 0, 0, fmt.Errorf("%w: page must be a positive integer", sharedErrors.ErrBadRequest)
		}
		page = parsed
	}
	if limitValue != "" {
		parsed, err := strconv.Atoi(limitValue)
		if err != nil || parsed < 1 {
			return 0, 0, fmt.Errorf("%w: limit must be a positive integer", sharedErrors.ErrBadRequest)
		}
		limit = normalizeDiscussionPageSize(parsed)
	}
	return page, limit, nil
}
