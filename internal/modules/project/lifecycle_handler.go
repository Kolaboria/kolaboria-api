package project

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
)

// ChangeStatus unpublishes an unused open project or archives it with history intact.
// @Summary Change project status
// @Tags Projects
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project UUID"
// @Param request body ChangeProjectStatusRequest true "Current version and target draft, in_progress, or archived"
// @Success 200 {object} response.SuccessResponse{data=ProjectDetailResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /api/v1/projects/{id}/status [patch]
func (h *ProjectHandler) ChangeStatus(c *gin.Context) {
	actorID, ok := requiredActorID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil || projectID == uuid.Nil {
		response.Error(c, http.StatusBadRequest, "Invalid project identifier")
		return
	}
	var request ChangeProjectStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid project status request")
		return
	}
	result, err := h.service.ChangeStatus(c.Request.Context(), actorID, projectID, request)
	if err != nil {
		writeProjectError(c, err)
		return
	}
	response.OK(c, "Project status updated", result)
}

// DeleteDraft permanently removes a draft without collaboration history.
// @Summary Delete project draft
// @Tags Projects
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project UUID"
// @Param request body DeleteProjectDraftRequest true "Current project version"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /api/v1/projects/{id} [delete]
func (h *ProjectHandler) DeleteDraft(c *gin.Context) {
	actorID, ok := requiredActorID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication is required")
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil || projectID == uuid.Nil {
		response.Error(c, http.StatusBadRequest, "Invalid project identifier")
		return
	}
	var request DeleteProjectDraftRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid project delete request")
		return
	}
	if err := h.service.DeleteDraft(c.Request.Context(), actorID, projectID, request); err != nil {
		writeProjectError(c, err)
		return
	}
	response.OK(c, "Project draft deleted", nil)
}
