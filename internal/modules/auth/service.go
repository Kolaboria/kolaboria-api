package auth

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/config"
	infraJwt "github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/jwt"
	infraMail "github.com/syahrulfahmi/kolaboria-api/internal/infrastructure/mail"
	sharedErrors "github.com/syahrulfahmi/kolaboria-api/internal/shared/errors"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) (*RegistrationResponse, error)
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	Logout(ctx context.Context, refreshToken string) error
	RefreshToken(ctx context.Context, req RefreshTokenRequest) (*AuthResponse, error)
	VerifyEmail(ctx context.Context, token string) error
	ResendVerification(ctx context.Context, req ResendVerificationRequest) (*VerificationResendResponse, error)
	ForgotPassword(ctx context.Context, req ForgotPasswordRequest) error
	ResetPassword(ctx context.Context, req ResetPasswordRequest) error
	GetCurrentUser(ctx context.Context, userID uuid.UUID) (*CurrentUserResponse, error)
	GoogleLogin(ctx context.Context, req GoogleCallbackRequest) (*LoginResponse, error)
}

const verificationResendCooldown = time.Minute

type authService struct {
	repo       AuthRepository
	jwtService infraJwt.JWTService
	mailer     infraMail.Mailer
	config     *config.Config
}

func NewAuthService(repo AuthRepository, jwtSvc infraJwt.JWTService, mailer infraMail.Mailer, cfg *config.Config) AuthService {
	return &authService{
		repo:       repo,
		jwtService: jwtSvc,
		mailer:     mailer,
		config:     cfg,
	}
}

func (s *authService) Register(ctx context.Context, req RegisterRequest) (*RegistrationResponse, error) {
	// 1. Check if email already registered
	_, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err == nil {
		return nil, sharedErrors.ErrEmailAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 2. Generate unique username
	username, err := s.generateUniqueUsername(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	// 3. Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// 4. Create user entity with associated profile
	userID := uuid.New()
	user := &User{
		ID:           userID,
		Email:        req.Email,
		PasswordHash: string(passwordHash),
		IsActive:     true,
		Profile: &Profile{
			ID:          userID,
			Username:    username,
			FullName:    req.Name,
			AccountType: nil, // not onboarded yet
		},
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	// 5. Generate email verification token (random secure string, valid for 24 hours)
	tokenStr := randomString(32)
	verificationToken := &UserToken{
		UserID:    user.ID,
		Token:     tokenStr,
		Type:      TokenTypeEmailVerification,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := s.repo.CreateToken(ctx, verificationToken); err != nil {
		return nil, err
	}

	availableAt := verificationToken.CreatedAt.Add(verificationResendCooldown).UTC()
	if verificationToken.CreatedAt.IsZero() {
		availableAt = time.Now().Add(verificationResendCooldown).UTC()
	}

	// 6. Send verification email in background
	verificationURL := s.verificationURL(tokenStr)
	go func() {
		_ = s.mailer.SendVerificationEmail(user.Email, user.GetFullName(), verificationURL)
	}()

	userResp := ToUserResponseWithoutProfile(user)
	return &RegistrationResponse{
		UserResponse:                  userResp,
		VerificationResendAvailableAt: availableAt,
	}, nil
}

func (s *authService) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	// 1. Find user by email
	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, sharedErrors.ErrInvalidCredentials
		}
		return nil, err
	}

	// 2. Verify status
	if !user.IsActive {
		return nil, sharedErrors.ErrAccountInactive
	}

	// 3. Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, sharedErrors.ErrInvalidCredentials
	}

	// 4. Generate access token
	var username string
	if user.Profile != nil {
		username = user.Profile.Username
	}
	accessToken, err := s.jwtService.GenerateAccessToken(user.ID, user.Email, username, "user")
	if err != nil {
		return nil, err
	}

	// 5. Generate & save refresh token
	refreshTokenStr := randomString(64)
	refreshToken := &UserToken{
		UserID:    user.ID,
		Token:     refreshTokenStr,
		Type:      TokenTypeRefresh,
		ExpiresAt: time.Now().Add(s.config.JWT.RefreshTokenExpiry),
	}

	if err := s.repo.CreateToken(ctx, refreshToken); err != nil {
		return nil, err
	}

	resp := ToLoginResponse(user, accessToken, refreshTokenStr)
	return &resp, nil
}

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	return s.repo.DeleteToken(ctx, refreshToken)
}

func (s *authService) RefreshToken(ctx context.Context, req RefreshTokenRequest) (*AuthResponse, error) {
	// 1. Find token
	token, err := s.repo.FindToken(ctx, req.RefreshToken, TokenTypeRefresh)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, sharedErrors.ErrTokenInvalid
		}
		return nil, err
	}

	// 2. Validate token
	if token.ExpiresAt.Before(time.Now()) {
		_ = s.repo.DeleteToken(ctx, req.RefreshToken)
		return nil, sharedErrors.ErrTokenExpired
	}

	// 3. Find user
	user, err := s.repo.FindUserByID(ctx, token.UserID)
	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		return nil, sharedErrors.ErrAccountInactive
	}

	// 4. Generate new access token
	var username string
	if user.Profile != nil {
		username = user.Profile.Username
	}
	newAccessToken, err := s.jwtService.GenerateAccessToken(user.ID, user.Email, username, "user")
	if err != nil {
		return nil, err
	}

	resp := ToAuthResponse(user, newAccessToken, token.Token)
	return &resp, nil
}

func (s *authService) VerifyEmail(ctx context.Context, tokenStr string) error {
	// 1. Find token
	token, err := s.repo.FindToken(ctx, tokenStr, TokenTypeEmailVerification)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sharedErrors.ErrTokenInvalid
		}
		return err
	}

	// 2. Validate token
	if token.UsedAt != nil {
		return sharedErrors.ErrTokenAlreadyUsed
	}
	if token.ExpiresAt.Before(time.Now()) {
		return sharedErrors.ErrTokenExpired
	}

	// 3. Find user
	user, err := s.repo.FindUserByID(ctx, token.UserID)
	if err != nil {
		return err
	}

	// 4. Update user email_verified_at
	now := time.Now()
	user.EmailVerifiedAt = &now
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return err
	}

	// 5. Mark token used
	return s.repo.MarkTokenUsed(ctx, token.ID)
}

func (s *authService) ResendVerification(ctx context.Context, req ResendVerificationRequest) (*VerificationResendResponse, error) {
	// 1. Find user
	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, sharedErrors.ErrNotFound
		}
		return nil, err
	}

	if user.EmailVerifiedAt != nil {
		return nil, sharedErrors.ErrBadRequest // Already verified
	}

	var availableAt time.Time
	var tokenStr string

	err = s.repo.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		var latestToken UserToken
		err := tx.Where("user_id = ? AND type = ? AND used_at IS NULL", user.ID, TokenTypeEmailVerification).
			Order("created_at DESC").
			First(&latestToken).Error

		if err == nil {
			nextAvailable := latestToken.CreatedAt.Add(verificationResendCooldown)
			if now.Before(nextAvailable) {
				retryAfter := int(nextAvailable.Sub(now).Seconds())
				if retryAfter < 1 {
					retryAfter = 1
				}
				return sharedErrors.NewVerificationCooldownError(nextAvailable.UTC(), retryAfter)
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// Delete old verification tokens
		if err := tx.Where("user_id = ? AND type = ?", user.ID, TokenTypeEmailVerification).Delete(&UserToken{}).Error; err != nil {
			return err
		}

		// Create new token
		tokenStr = randomString(32)
		newToken := &UserToken{
			UserID:    user.ID,
			Token:     tokenStr,
			Type:      TokenTypeEmailVerification,
			ExpiresAt: now.Add(24 * time.Hour),
			CreatedAt: now,
		}

		if err := tx.Create(newToken).Error; err != nil {
			return err
		}

		availableAt = newToken.CreatedAt.Add(verificationResendCooldown).UTC()
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Send verification email
	verificationURL := s.verificationURL(tokenStr)
	go func() {
		_ = s.mailer.SendVerificationEmail(user.Email, user.GetFullName(), verificationURL)
	}()

	return &VerificationResendResponse{
		VerificationResendAvailableAt: availableAt,
	}, nil
}

func (s *authService) ForgotPassword(ctx context.Context, req ForgotPasswordRequest) error {
	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Don't leak user existence; return success anyways
			return nil
		}
		return err
	}

	// Delete old reset tokens
	_ = s.repo.DeleteUserTokensByType(ctx, user.ID, TokenTypePasswordReset)

	// Create reset password token (expires in 15 minutes)
	tokenStr := randomString(32)
	resetToken := &UserToken{
		UserID:    user.ID,
		Token:     tokenStr,
		Type:      TokenTypePasswordReset,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	if err := s.repo.CreateToken(ctx, resetToken); err != nil {
		return err
	}

	frontendURL := strings.TrimRight(s.config.App.FrontendURL, "/")
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", frontendURL, url.QueryEscape(tokenStr))
	go func() {
		_ = s.mailer.SendResetPasswordEmail(user.Email, user.GetFullName(), resetURL)
	}()

	return nil
}

func (s *authService) ResetPassword(ctx context.Context, req ResetPasswordRequest) error {
	token, err := s.repo.FindToken(ctx, req.Token, TokenTypePasswordReset)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sharedErrors.ErrTokenInvalid
		}
		return err
	}

	if token.UsedAt != nil {
		return sharedErrors.ErrTokenAlreadyUsed
	}
	if token.ExpiresAt.Before(time.Now()) {
		return sharedErrors.ErrTokenExpired
	}

	user, err := s.repo.FindUserByID(ctx, token.UserID)
	if err != nil {
		return err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(passwordHash)
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return err
	}

	return s.repo.MarkTokenUsed(ctx, token.ID)
}

func (s *authService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*CurrentUserResponse, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, sharedErrors.ErrNotFound
		}
		return nil, err
	}

	resp := ToCurrentUserResponse(user)
	return &resp, nil
}

func (s *authService) verificationURL(token string) string {
	frontendURL := strings.TrimRight(s.config.App.FrontendURL, "/")
	return fmt.Sprintf("%s/verify-email?token=%s", frontendURL, url.QueryEscape(token))
}

func toKebabCase(s string) string {
	s = strings.ToLower(s)
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			sb.WriteRune('-')
		}
	}
	res := sb.String()
	for strings.Contains(res, "--") {
		res = strings.ReplaceAll(res, "--", "-")
	}
	return strings.Trim(res, "-")
}

func randomString(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

func (s *authService) generateUniqueUsername(ctx context.Context, name string) (string, error) {
	base := toKebabCase(name)
	if base == "" {
		base = "user"
	}

	_, err := s.repo.FindUserByUsername(ctx, base)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return base, nil
	}
	if err != nil {
		return "", err
	}

	for i := 0; i < 10; i++ {
		suffix := randomString(4)
		candidate := fmt.Sprintf("%s-%s", base, suffix)
		_, err = s.repo.FindUserByUsername(ctx, candidate)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%s-%s", base, randomString(8)), nil
}

func (s *authService) GoogleLogin(ctx context.Context, req GoogleCallbackRequest) (*LoginResponse, error) {
	oauthConf := &oauth2.Config{
		ClientID:     s.config.Google.ClientID,
		ClientSecret: s.config.Google.ClientSecret,
		RedirectURL:  s.config.Google.RedirectURI,
		Endpoint:     google.Endpoint,
	}

	token, err := oauthConf.Exchange(ctx, req.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	client := oauthConf.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	var googleUser struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	var user *User
	user, err = s.repo.FindUserByGoogleID(ctx, googleUser.Sub)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if user == nil {
		user, err = s.repo.FindUserByEmail(ctx, googleUser.Email)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		if user != nil {
			user.GoogleID = &googleUser.Sub
			user.Provider = "google"
			if err := s.repo.UpdateUser(ctx, user); err != nil {
				return nil, err
			}
		}
	}

	if user == nil {
		username, err := s.generateUniqueUsername(ctx, googleUser.Name)
		if err != nil {
			return nil, err
		}

		randomPassword := randomString(32)
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(randomPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}

		userID := uuid.New()
		now := time.Now()

		var avatarPtr *string
		if googleUser.Picture != "" {
			avatarVal := googleUser.Picture
			avatarPtr = &avatarVal
		}

		user = &User{
			ID:              userID,
			Email:           googleUser.Email,
			PasswordHash:    string(passwordHash),
			EmailVerifiedAt: &now,
			IsActive:        true,
			GoogleID:        &googleUser.Sub,
			Provider:        "google",
			Profile: &Profile{
				ID:          userID,
				Username:    username,
				FullName:    googleUser.Name,
				Avatar:      avatarPtr,
				AccountType: nil,
			},
		}

		if err := s.repo.CreateUser(ctx, user); err != nil {
			return nil, err
		}
	}

	if user != nil && user.Profile != nil && googleUser.Picture != "" {
		if user.Profile.Avatar == nil || *user.Profile.Avatar == "" {
			avatarVal := googleUser.Picture
			user.Profile.Avatar = &avatarVal
			if err := s.repo.GetDB().WithContext(ctx).Save(user.Profile).Error; err != nil {
				log.Printf("[GoogleLogin] failed to update user avatar: %v", err)
			}
		}
	}

	if !user.IsActive {
		return nil, sharedErrors.ErrAccountInactive
	}

	var username string
	if user.Profile != nil {
		username = user.Profile.Username
	}
	accessToken, err := s.jwtService.GenerateAccessToken(user.ID, user.Email, username, "user")
	if err != nil {
		return nil, err
	}

	refreshTokenStr := randomString(64)
	refreshToken := &UserToken{
		UserID:    user.ID,
		Token:     refreshTokenStr,
		Type:      TokenTypeRefresh,
		ExpiresAt: time.Now().Add(s.config.JWT.RefreshTokenExpiry),
	}

	if err := s.repo.CreateToken(ctx, refreshToken); err != nil {
		return nil, err
	}

	loginResp := ToLoginResponse(user, accessToken, refreshTokenStr)
	return &loginResp, nil
}
