package auth

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/internal/http/response"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
)

type AuthHandler struct {
	service   AuthService
	validator *validator.Validate
}

func NewAuthHandler(service AuthService) *AuthHandler {
	return &AuthHandler{
		service:   service,
		validator: validator.New(),
	}
}

// Register handles user registration.
// @Summary      Register
// @Description  Register a new user account
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body RegisterRequest true "Register Request"
// @Success      201  {object}  response.SuccessResponse{data=RegistrationResponse}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      409  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /v1/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.validator.Struct(req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	resp, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, sharedErrors.ErrEmailAlreadyExists) {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "User registered successfully", resp)
}

// Login handles user login.
// @Summary      Login
// @Description  Authenticate user and return tokens
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body LoginRequest true "Login Request"
// @Success      200  {object}  response.SuccessResponse{data=LoginResponse}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.validator.Struct(req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	resp, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, sharedErrors.ErrInvalidCredentials) {
			response.Error(c, http.StatusUnauthorized, err.Error())
			return
		}
		if errors.Is(err, sharedErrors.ErrAccountInactive) {
			response.Error(c, http.StatusForbidden, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Login successful", resp)
}

// Logout handles user logout by revoking refresh token.
// @Summary      Logout
// @Description  Revoke refresh token and end session
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body RefreshTokenRequest true "Logout Request"
// @Success      200  {object}  response.SuccessResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /v1/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.validator.Struct(req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	if err := h.service.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Logged out successfully", nil)
}

// RefreshToken handles access token refresh.
// @Summary      Refresh Token
// @Description  Get a new access token using a valid refresh token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body RefreshTokenRequest true "Refresh Token Request"
// @Success      200  {object}  response.SuccessResponse{data=AuthResponse}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.validator.Struct(req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	resp, err := h.service.RefreshToken(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, sharedErrors.ErrTokenInvalid) || errors.Is(err, sharedErrors.ErrTokenExpired) {
			response.Error(c, http.StatusUnauthorized, err.Error())
			return
		}
		if errors.Is(err, sharedErrors.ErrAccountInactive) {
			response.Error(c, http.StatusForbidden, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Token refreshed successfully", resp)
}

// VerifyEmail handles email verification token from URL query.
// @Summary      Verify Email
// @Description  Verify user email using token
// @Tags         auth
// @Produce      json
// @Param        token query string true "Verification Token"
// @Success      200  {object}  response.SuccessResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /v1/auth/verify-email [get]
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		response.Error(c, http.StatusBadRequest, "Token is required")
		return
	}

	if err := h.service.VerifyEmail(c.Request.Context(), token); err != nil {
		if errors.Is(err, sharedErrors.ErrTokenInvalid) || errors.Is(err, sharedErrors.ErrTokenExpired) || errors.Is(err, sharedErrors.ErrTokenAlreadyUsed) {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Email verified successfully", nil)
}

// ResendVerification handles resending verification email.
// @Summary      Resend Verification
// @Description  Resend email verification link with 60s cooldown
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body ResendVerificationRequest true "Resend Verification Request"
// @Success      200  {object}  response.SuccessResponse{data=VerificationResendResponse}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Failure      429  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /v1/auth/resend-verification [post]
func (h *AuthHandler) ResendVerification(c *gin.Context) {
	var req ResendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.validator.Struct(req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	resp, err := h.service.ResendVerification(c.Request.Context(), req)
	if err != nil {
		var cooldownErr *sharedErrors.VerificationCooldownError
		if errors.As(err, &cooldownErr) {
			c.Header("Retry-After", strconv.Itoa(cooldownErr.RetryAfterSeconds))
			response.Error(c, http.StatusTooManyRequests, err.Error())
			return
		}
		if errors.Is(err, sharedErrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "User not found")
			return
		}
		if errors.Is(err, sharedErrors.ErrBadRequest) {
			response.Error(c, http.StatusBadRequest, "Email already verified")
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Verification email resent successfully", resp)
}

// ForgotPassword handles sending password reset email.
// @Summary      Forgot Password
// @Description  Send password reset link to user email
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body ForgotPasswordRequest true "Forgot Password Request"
// @Success      200  {object}  response.SuccessResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /v1/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.validator.Struct(req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	if err := h.service.ForgotPassword(c.Request.Context(), req); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "If your email is registered, you will receive a password reset link", nil)
}

// ResetPassword handles resetting password with token.
// @Summary      Reset Password
// @Description  Reset user password using valid token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body ResetPasswordRequest true "Reset Password Request"
// @Success      200  {object}  response.SuccessResponse
// @Failure      400  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /v1/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.validator.Struct(req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	if err := h.service.ResetPassword(c.Request.Context(), req); err != nil {
		if errors.Is(err, sharedErrors.ErrTokenInvalid) || errors.Is(err, sharedErrors.ErrTokenExpired) || errors.Is(err, sharedErrors.ErrTokenAlreadyUsed) {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Password reset successfully", nil)
}

// GoogleLogin handles Google OAuth callback.
// @Summary      Google Login
// @Description  Authenticate via Google OAuth authorization code
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body GoogleCallbackRequest true "Google Callback Request"
// @Success      200  {object}  response.SuccessResponse{data=LoginResponse}
// @Failure      400  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /v1/auth/google [post]
func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	var req GoogleCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.validator.Struct(req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	resp, err := h.service.GoogleLogin(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, sharedErrors.ErrAccountInactive) {
			response.Error(c, http.StatusForbidden, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Login with Google successful", resp)
}

// Me retrieves the current authenticated user profile.
// @Summary      Get Current User
// @Description  Get current account identity, system role, and authorized organization choices from persisted state
// @Description  Login with POST /v1/auth/login, copy data.access_token, and set Swagger Authorize to Bearer <access_token>. Then use Try it out on this GET endpoint; no request body is required.
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.SuccessResponse{data=CurrentUserResponse}
// @Failure      401  {object}  response.ErrorResponse
// @Failure      403  {object}  response.ErrorResponse
// @Failure      404  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /v1/auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "Invalid user ID")
		return
	}

	resp, err := h.service.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, sharedErrors.ErrAccountInactive) {
			response.Error(c, http.StatusForbidden, "Account is inactive")
			return
		}
		if errors.Is(err, sharedErrors.ErrNotFound) {
			response.Error(c, http.StatusNotFound, "User not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve account context")
		return
	}

	response.Success(c, http.StatusOK, "User retrieved successfully", resp)
}
