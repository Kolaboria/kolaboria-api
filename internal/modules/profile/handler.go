package profile

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

type ProfileHandler struct {
	service   ProfileUseCase
	validator *validator.Validate
}

func NewProfileHandler(service ProfileUseCase) *ProfileHandler {
	return &ProfileHandler{service: service, validator: validator.New()}
}

// CheckOnboardingStatus returns whether the authenticated user's profile has completed onboarding.
// @Summary      Check onboarding status
// @Description  Check whether the authenticated user's profile has an account type.
// @Tags         profile
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.SuccessResponse{data=OnboardingStatusResponse}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /v1/profiles/onboarding/status [get]
func (h *ProfileHandler) CheckOnboardingStatus(c *gin.Context) {
	userID, ok := authenticatedProfileUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	status, err := h.service.CheckOnboardingStatus(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, sharedErrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "User profile not found")
			return
		}
		slog.ErrorContext(c.Request.Context(), "failed to check onboarding status", "user_id", userID, "error", err)
		response.Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}

	response.Success(c, http.StatusOK, "Onboarding status retrieved successfully", status)
}

// SubmitOnboarding saves a talent's initial profile details and master-data selections.
// @Summary      Submit talent onboarding
// @Description  Save talent profile details, primary skill, and selected tools.
// @Tags         profile
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body SubmitOnboardingRequest true "Onboarding request"
// @Success      200  {object}  response.SuccessResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /v1/profiles/onboarding [post]
func (h *ProfileHandler) SubmitOnboarding(c *gin.Context) {
	userID, ok := authenticatedProfileUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var request SubmitOnboardingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid onboarding request")
		return
	}
	request.Goal = strings.TrimSpace(request.Goal)
	request.Bio = strings.TrimSpace(request.Bio)
	request.ExternalLink = strings.TrimSpace(request.ExternalLink)
	if err := h.validator.Struct(request); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	if err := h.service.SubmitOnboarding(c.Request.Context(), userID, request); err != nil {
		switch {
		case errors.Is(err, sharedErrors.ErrBadRequest):
			response.Error(c, http.StatusBadRequest, "Invalid onboarding data")
		case errors.Is(err, sharedErrors.ErrNotFound):
			response.Error(c, http.StatusNotFound, "User profile not found")
		default:
			slog.ErrorContext(c.Request.Context(), "failed to submit onboarding", "user_id", userID, "error", err)
			response.Error(c, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	response.Success(c, http.StatusOK, "Onboarding submitted successfully", nil)
}

func authenticatedProfileUserID(c *gin.Context) (uuid.UUID, bool) {
	userIDValue, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, false
	}
	userIDString, ok := userIDValue.(string)
	if !ok {
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(userIDString)
	if err != nil {
		return uuid.Nil, false
	}
	return userID, true
}
