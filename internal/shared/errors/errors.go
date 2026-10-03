package errors

import (
	"errors"
	"time"
)

var (
	ErrNotFound           = errors.New("resource not found")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrBadRequest         = errors.New("bad request")
	ErrConflict           = errors.New("conflict")
	ErrSlugConflict       = errors.New("slug already taken")
	ErrInternalServer     = errors.New("internal server error")
	ErrServiceUnavailable = errors.New("service unavailable")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailAlreadyExists = errors.New("email already registered")
	ErrTokenExpired       = errors.New("token has expired")
	ErrTokenInvalid       = errors.New("token is invalid")
	ErrTokenAlreadyUsed   = errors.New("token has already been used")
	ErrAccountInactive    = errors.New("account is inactive")
)

type VerificationCooldownError struct {
	AvailableAt       time.Time
	RetryAfterSeconds int
}

func (e *VerificationCooldownError) Error() string {
	return "verification email cooldown active"
}

func NewVerificationCooldownError(availableAt time.Time, retryAfterSeconds int) *VerificationCooldownError {
	return &VerificationCooldownError{
		AvailableAt:       availableAt,
		RetryAfterSeconds: retryAfterSeconds,
	}
}
