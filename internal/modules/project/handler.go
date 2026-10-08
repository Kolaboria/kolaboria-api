package project

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

type ProjectHandler struct{ service *ProjectService }

func NewProjectHandler(service *ProjectService) *ProjectHandler {
	return &ProjectHandler{service: service}
}

// CreateDraft creates a persisted project draft.
// @Summary Create project draft
// @Tags Projects
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateProjectDraftRequest true "Project draft"
// @Success 201 {object} response.SuccessResponse{data=ProjectDetailResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /api/v1/projects [post]
func (h *ProjectHandler) CreateDraft(c *gin.Context) {
	actorID, ok := requiredActorID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
		return
	}
	var request CreateProjectDraftRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid project request")
		return
	}
	result, err := h.service.CreateDraft(c.Request.Context(), actorID, request)
	if err != nil {
		writeProjectError(c, err)
		return
	}
	status := http.StatusOK
	message := "Project draft already exists"
	if result.created {
		status = http.StatusCreated
		message = "Project draft created"
	}
	response.Success(c, status, message, result)
}

// UpdateDefinition replaces the editable project definition with optimistic version checking.
// @Summary Update project definition
// @Tags Projects
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project UUID"
// @Param request body UpdateProjectDefinitionRequest true "Project definition"
// @Success 200 {object} response.SuccessResponse{data=ProjectDetailResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /api/v1/projects/{id} [put]
func (h *ProjectHandler) UpdateDefinition(c *gin.Context) {
	actorID, ok := requiredActorID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid project identifier")
		return
	}
	var request UpdateProjectDefinitionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid project request")
		return
	}
	result, err := h.service.UpdateDefinition(c.Request.Context(), actorID, projectID, request)
	if err != nil {
		writeProjectError(c, err)
		return
	}
	response.OK(c, "Project updated", result)
}

// Publish changes a complete project draft to its first published state.
// @Summary Publish project
// @Tags Projects
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project UUID"
// @Param request body PublishProjectRequest true "Current project version"
// @Success 200 {object} response.SuccessResponse{data=ProjectDetailResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /api/v1/projects/{id}/publish [post]
func (h *ProjectHandler) Publish(c *gin.Context) {
	actorID, ok := requiredActorID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid project identifier")
		return
	}
	var request PublishProjectRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid project request")
		return
	}
	result, err := h.service.Publish(c.Request.Context(), actorID, projectID, request)
	if err != nil {
		writeProjectError(c, err)
		return
	}
	response.OK(c, "Project published", result)
}

// GetByID retrieves a project visible to the current actor or public discovery.
// @Summary Get project by ID
// @Tags Projects
// @Produce json
// @Param id path string true "Project UUID"
// @Success 200 {object} response.SuccessResponse{data=ProjectDetailResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{id} [get]
func (h *ProjectHandler) GetByID(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid project identifier")
		return
	}
	result, err := h.service.GetByID(c.Request.Context(), optionalActorID(c), projectID)
	if err != nil {
		writeProjectError(c, err)
		return
	}
	response.OK(c, "Project retrieved", result)
}

// GetBySlug retrieves a project visible to the current actor or public discovery.
// @Summary Get project by slug
// @Tags Projects
// @Produce json
// @Param slug path string true "Project slug"
// @Success 200 {object} response.SuccessResponse{data=ProjectDetailResponse}
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/slug/{slug} [get]
func (h *ProjectHandler) GetBySlug(c *gin.Context) {
	result, err := h.service.GetBySlug(c.Request.Context(), optionalActorID(c), c.Param("slug"))
	if err != nil {
		writeProjectError(c, err)
		return
	}
	response.OK(c, "Project retrieved", result)
}

// ListPublic returns public published projects.
// @Summary List public projects
// @Tags Projects
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Items per page (max 100)"
// @Param q query string false "Search title and summary"
// @Success 200 {object} response.SuccessResponse{data=[]ProjectSummaryResponse,meta=ProjectPaginationMeta}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/projects [get]
func (h *ProjectHandler) ListPublic(c *gin.Context) {
	filter, ok := parseProjectListFilter(c)
	if !ok {
		response.Error(c, http.StatusBadRequest, "Invalid project filters")
		return
	}
	items, meta, err := h.service.ListPublic(c.Request.Context(), filter)
	if err != nil {
		writeProjectError(c, err)
		return
	}
	response.SuccessWithMeta(c, http.StatusOK, "Projects retrieved", items, meta)
}

// ListMine returns projects owned by the current actor or initiated organizations.
// @Summary List my projects
// @Tags Projects
// @Produce json
// @Security BearerAuth
// @Param scope query string false "owned or initiated"
// @Param page query int false "Page number"
// @Param limit query int false "Items per page (max 100)"
// @Param q query string false "Search title and summary"
// @Success 200 {object} response.SuccessResponse{data=[]ProjectSummaryResponse,meta=ProjectPaginationMeta}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /api/v1/projects/my-projects [get]
func (h *ProjectHandler) ListMine(c *gin.Context) {
	actorID, ok := requiredActorID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
		return
	}
	filter, ok := parseProjectListFilter(c)
	if !ok {
		response.Error(c, http.StatusBadRequest, "Invalid project filters")
		return
	}
	filter.Scope = c.Query("scope")
	items, meta, err := h.service.ListMine(c.Request.Context(), actorID, filter)
	if err != nil {
		writeProjectError(c, err)
		return
	}
	response.SuccessWithMeta(c, http.StatusOK, "My projects retrieved", items, meta)
}

func parseProjectListFilter(c *gin.Context) (ProjectListFilter, bool) {
	filter := ProjectListFilter{Page: 1, Limit: 20, Search: c.Query("q")}
	if raw := c.Query("page"); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil {
			return ProjectListFilter{}, false
		}
		filter.Page = page
	}
	if raw := c.Query("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			return ProjectListFilter{}, false
		}
		filter.Limit = limit
	}
	return filter, true
}

func requiredActorID(c *gin.Context) (uuid.UUID, bool) {
	actorID, err := uuid.Parse(c.GetString("user_id"))
	return actorID, err == nil && actorID != uuid.Nil
}

func optionalActorID(c *gin.Context) *uuid.UUID {
	actorID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil || actorID == uuid.Nil {
		return nil
	}
	return &actorID
}

func writeProjectError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sharedErrors.ErrUnauthorized):
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
	case errors.Is(err, sharedErrors.ErrForbidden):
		response.Error(c, http.StatusForbidden, "You are not allowed to perform this project action")
	case errors.Is(err, sharedErrors.ErrNotFound):
		response.Error(c, http.StatusNotFound, "Project not found")
	case errors.Is(err, sharedErrors.ErrBadRequest):
		response.Error(c, http.StatusBadRequest, "Project data is invalid")
	case errors.Is(err, sharedErrors.ErrConflict), errors.Is(err, sharedErrors.ErrSlugConflict):
		response.Error(c, http.StatusConflict, "Project changed or conflicts with existing data")
	default:
		response.Error(c, http.StatusInternalServerError, "Unable to process project request")
	}
}
