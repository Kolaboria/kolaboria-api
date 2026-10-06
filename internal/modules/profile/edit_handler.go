package profile

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

// GetMyProfile returns the authenticated user's editable profile.
// @Summary Get current profile
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.SuccessResponse{data=MyProfileDetailResponse}
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/profiles/me [get]
func (h *ProfileHandler) GetMyProfile(c *gin.Context) {
	userID, ok := authenticatedProfileUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	result, err := h.service.GetMyProfile(c.Request.Context(), userID)
	if err != nil {
		h.respondEditError(c, "get profile", err)
		return
	}
	response.Success(c, http.StatusOK, "Profile retrieved successfully", result)
}

// UpdateMyProfile applies a partial update to the authenticated user's profile.
// @Summary Update current profile
// @Tags profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateProfileRequest true "Partial profile update"
// @Success 200 {object} response.SuccessResponse{data=MyProfileDetailResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /api/v1/profiles/me [patch]
func (h *ProfileHandler) UpdateMyProfile(c *gin.Context) {
	userID, ok := authenticatedProfileUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var request UpdateProfileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid profile update")
		return
	}
	result, err := h.service.UpdateProfile(c.Request.Context(), userID, request)
	if err != nil {
		h.respondEditError(c, "update profile", err)
		return
	}
	response.Success(c, http.StatusOK, "Profile updated successfully", result)
}

// ListMySkills returns the authenticated user's skills.
// @Summary List current user's skills
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.SuccessResponse{data=[]UserSkillResponse}
// @Router /api/v1/profiles/me/skills [get]
func (h *ProfileHandler) ListMySkills(c *gin.Context) {
	userID, ok := authenticatedProfileUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	result, err := h.service.ListSkills(c.Request.Context(), userID)
	if err != nil {
		h.respondEditError(c, "list profile skills", err)
		return
	}
	response.Success(c, http.StatusOK, "Profile skills retrieved successfully", result)
}

// AddMySkill associates a master skill with the current profile.
// @Summary Add a skill to current profile
// @Tags profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body AddSkillRequest true "Skill selection"
// @Success 201 {object} response.SuccessResponse{data=UserSkillResponse}
// @Router /api/v1/profiles/me/skills [post]
func (h *ProfileHandler) AddMySkill(c *gin.Context) {
	userID, ok := authenticatedProfileUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var request AddSkillRequest
	if err := c.ShouldBindJSON(&request); err != nil || h.validator.Struct(request) != nil {
		response.Error(c, http.StatusBadRequest, "Invalid skill request")
		return
	}
	result, err := h.service.AddSkill(c.Request.Context(), userID, request)
	if err != nil {
		h.respondEditError(c, "add profile skill", err)
		return
	}
	response.Success(c, http.StatusCreated, "Profile skill added successfully", result)
}

// RemoveMySkill removes an owned skill association.
// @Summary Remove a skill from current profile
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Param id path string true "User skill ID"
// @Success 200 {object} response.SuccessResponse
// @Router /api/v1/profiles/me/skills/{id} [delete]
func (h *ProfileHandler) RemoveMySkill(c *gin.Context) {
	userID, id, ok := authenticatedProfilePathID(c)
	if !ok {
		return
	}
	if err := h.service.RemoveSkill(c.Request.Context(), userID, id); err != nil {
		h.respondEditError(c, "remove profile skill", err)
		return
	}
	response.Success(c, http.StatusOK, "Profile skill removed successfully", nil)
}

// SetPrimaryMySkill promotes an owned skill.
// @Summary Set current profile primary skill
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Param id path string true "User skill ID"
// @Success 200 {object} response.SuccessResponse
// @Router /api/v1/profiles/me/skills/{id}/primary [patch]
func (h *ProfileHandler) SetPrimaryMySkill(c *gin.Context) {
	userID, id, ok := authenticatedProfilePathID(c)
	if !ok {
		return
	}
	if err := h.service.SetPrimarySkill(c.Request.Context(), userID, id); err != nil {
		h.respondEditError(c, "set primary profile skill", err)
		return
	}
	response.Success(c, http.StatusOK, "Primary profile skill updated successfully", nil)
}

// ListMyTools returns the authenticated user's tools.
// @Summary List current user's tools
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.SuccessResponse{data=[]UserToolResponse}
// @Router /api/v1/profiles/me/tools [get]
func (h *ProfileHandler) ListMyTools(c *gin.Context) {
	userID, ok := authenticatedProfileUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	result, err := h.service.ListTools(c.Request.Context(), userID)
	if err != nil {
		h.respondEditError(c, "list profile tools", err)
		return
	}
	response.Success(c, http.StatusOK, "Profile tools retrieved successfully", result)
}

// AddMyTool associates a master tool with the current profile.
// @Summary Add a tool to current profile
// @Tags profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body AddToolRequest true "Tool selection"
// @Success 201 {object} response.SuccessResponse{data=UserToolResponse}
// @Router /api/v1/profiles/me/tools [post]
func (h *ProfileHandler) AddMyTool(c *gin.Context) {
	userID, ok := authenticatedProfileUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var request AddToolRequest
	if err := c.ShouldBindJSON(&request); err != nil || h.validator.Struct(request) != nil {
		response.Error(c, http.StatusBadRequest, "Invalid tool request")
		return
	}
	result, err := h.service.AddTool(c.Request.Context(), userID, request)
	if err != nil {
		h.respondEditError(c, "add profile tool", err)
		return
	}
	response.Success(c, http.StatusCreated, "Profile tool added successfully", result)
}

// RemoveMyTool removes an owned tool association.
// @Summary Remove a tool from current profile
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Param id path string true "User tool ID"
// @Success 200 {object} response.SuccessResponse
// @Router /api/v1/profiles/me/tools/{id} [delete]
func (h *ProfileHandler) RemoveMyTool(c *gin.Context) {
	userID, id, ok := authenticatedProfilePathID(c)
	if !ok {
		return
	}
	if err := h.service.RemoveTool(c.Request.Context(), userID, id); err != nil {
		h.respondEditError(c, "remove profile tool", err)
		return
	}
	response.Success(c, http.StatusOK, "Profile tool removed successfully", nil)
}

// ListMyCareers returns the authenticated user's career history.
// @Summary List current user's career history
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.SuccessResponse{data=[]CareerHistoryResponse}
// @Router /api/v1/profiles/me/careers [get]
func (h *ProfileHandler) ListMyCareers(c *gin.Context) {
	userID, ok := authenticatedProfileUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	result, err := h.service.ListCareerHistories(c.Request.Context(), userID)
	if err != nil {
		h.respondEditError(c, "list profile careers", err)
		return
	}
	response.Success(c, http.StatusOK, "Career histories retrieved successfully", result)
}

// CreateMyCareer stores a career history entry for the authenticated user.
// @Summary Add career history
// @Tags profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CareerHistoryRequest true "Career history"
// @Success 201 {object} response.SuccessResponse{data=CareerHistoryResponse}
// @Router /api/v1/profiles/me/careers [post]
func (h *ProfileHandler) CreateMyCareer(c *gin.Context) {
	userID, ok := authenticatedProfileUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var request CareerHistoryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid career history")
		return
	}
	result, err := h.service.CreateCareerHistory(c.Request.Context(), userID, request)
	if err != nil {
		h.respondEditError(c, "create career history", err)
		return
	}
	response.Success(c, http.StatusCreated, "Career history created successfully", result)
}

// UpdateMyCareer replaces an owned career history entry.
// @Summary Update career history
// @Tags profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Career history ID"
// @Param request body CareerHistoryRequest true "Career history"
// @Success 200 {object} response.SuccessResponse{data=CareerHistoryResponse}
// @Router /api/v1/profiles/me/careers/{id} [put]
func (h *ProfileHandler) UpdateMyCareer(c *gin.Context) {
	userID, id, ok := authenticatedProfilePathID(c)
	if !ok {
		return
	}
	var request CareerHistoryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid career history")
		return
	}
	result, err := h.service.UpdateCareerHistory(c.Request.Context(), userID, id, request)
	if err != nil {
		h.respondEditError(c, "update career history", err)
		return
	}
	response.Success(c, http.StatusOK, "Career history updated successfully", result)
}

// DeleteMyCareer removes an owned career history entry.
// @Summary Delete career history
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Param id path string true "Career history ID"
// @Success 200 {object} response.SuccessResponse
// @Router /api/v1/profiles/me/careers/{id} [delete]
func (h *ProfileHandler) DeleteMyCareer(c *gin.Context) {
	userID, id, ok := authenticatedProfilePathID(c)
	if !ok {
		return
	}
	if err := h.service.DeleteCareerHistory(c.Request.Context(), userID, id); err != nil {
		h.respondEditError(c, "delete career history", err)
		return
	}
	response.Success(c, http.StatusOK, "Career history deleted successfully", nil)
}

func authenticatedProfilePathID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	userID, ok := authenticatedProfileUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid resource ID")
		return uuid.Nil, uuid.Nil, false
	}
	return userID, id, true
}

func (h *ProfileHandler) respondEditError(c *gin.Context, operation string, err error) {
	switch {
	case errors.Is(err, sharedErrors.ErrUnauthorized):
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
	case errors.Is(err, sharedErrors.ErrBadRequest):
		response.Error(c, http.StatusBadRequest, "Invalid profile data")
	case errors.Is(err, sharedErrors.ErrNotFound):
		response.Error(c, http.StatusNotFound, "Profile data not found")
	case errors.Is(err, sharedErrors.ErrConflict):
		response.Error(c, http.StatusConflict, "Profile data conflicts with an existing value")
	default:
		slog.ErrorContext(c.Request.Context(), "profile edit request failed", "operation", operation, "user_id", c.GetString("user_id"), "error", err)
		response.Error(c, http.StatusInternalServerError, "Internal server error")
	}
}
