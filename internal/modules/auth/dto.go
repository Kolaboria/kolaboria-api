package auth

import (
	"time"

	"github.com/google/uuid"
)

type RegisterRequest struct {
	Name            string `json:"name" validate:"required,min=2,max=100"`
	Email           string `json:"email" validate:"required,email"`
	Password        string `json:"password" validate:"required,min=8"`
	ConfirmPassword string `json:"confirmPassword" validate:"required,eqfield=Password"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" validate:"required"`
}

type ResendVerificationRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ResetPasswordRequest struct {
	Token           string `json:"token" validate:"required"`
	Password        string `json:"password" validate:"required,min=8"`
	ConfirmPassword string `json:"confirmPassword" validate:"required,eqfield=Password"`
}

type GoogleCallbackRequest struct {
	Code string `json:"code" validate:"required"`
}

type ProfileResponse struct {
	AccountType *string   `json:"account_type"`
	Avatar      *string   `json:"avatar"`
	Headline    *string   `json:"headline"`
	Bio         *string   `json:"bio"`
	Location    *string   `json:"location"`
	IsVerified  bool      `json:"is_verified"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type UserResponse struct {
	ID              uuid.UUID        `json:"id"`
	Name            string           `json:"name"`
	Username        string           `json:"username"`
	Email           string           `json:"email"`
	EmailVerifiedAt *time.Time       `json:"email_verified_at"`
	IsActive        bool             `json:"is_active"`
	CreatedAt       time.Time        `json:"created_at"`
	Profile         *ProfileResponse `json:"profile,omitempty"`
}

type RegistrationResponse struct {
	UserResponse
	VerificationResendAvailableAt time.Time `json:"verification_resend_available_at"`
}

type VerificationResendResponse struct {
	VerificationResendAvailableAt time.Time `json:"verification_resend_available_at"`
}

type AuthResponse struct {
	AccessToken                   string       `json:"access_token"`
	RefreshToken                  string       `json:"refresh_token"`
	User                          UserResponse `json:"user"`
	VerificationResendAvailableAt *time.Time   `json:"verification_resend_available_at,omitempty"`
}

type LoginResponse struct {
	AccessToken     string     `json:"access_token"`
	RefreshToken    string     `json:"refresh_token"`
	Name            string     `json:"name"`
	Email           string     `json:"email"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
}

type CurrentUserProfileResp struct {
	AccountType *string   `json:"account_type"`
	Avatar      *string   `json:"avatar"`
	Headline    *string   `json:"headline"`
	Bio         *string   `json:"bio"`
	Location    *string   `json:"location"`
	IsVerified  bool      `json:"is_verified"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CurrentUserResponse struct {
	Name            string                  `json:"name"`
	Email           string                  `json:"email"`
	EmailVerifiedAt *time.Time              `json:"email_verified_at"`
	IsActive        bool                    `json:"is_active"`
	CreatedAt       time.Time               `json:"created_at"`
	Profile         *CurrentUserProfileResp `json:"profile,omitempty"`
}
