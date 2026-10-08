package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuthRepository interface {
	CreateUser(ctx context.Context, user *User) error
	FindUserByEmail(ctx context.Context, email string) (*User, error)
	FindUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	ListInitiableOrganizations(ctx context.Context, userID uuid.UUID) ([]InitiableOrganizationResponse, error)
	FindUserByGoogleID(ctx context.Context, googleID string) (*User, error)
	FindUserByUsername(ctx context.Context, username string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	GetDB() *gorm.DB

	CreateToken(ctx context.Context, token *UserToken) error
	FindToken(ctx context.Context, token string, tokenType TokenType) (*UserToken, error)
	MarkTokenUsed(ctx context.Context, tokenID uuid.UUID) error
	DeleteToken(ctx context.Context, token string) error
	DeleteUserTokensByType(ctx context.Context, userID uuid.UUID, tokenType TokenType) error
	GetLatestUnusedToken(ctx context.Context, userID uuid.UUID, tokenType TokenType) (*UserToken, error)
}

type authRepository struct {
	db *gorm.DB
}

func NewAuthRepository(db *gorm.DB) AuthRepository {
	return &authRepository{
		db: db,
	}
}

func (r *authRepository) GetDB() *gorm.DB {
	return r.db
}

func (r *authRepository) CreateUser(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *authRepository) FindUserByGoogleID(ctx context.Context, googleID string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).Preload("Profile").Where("google_id = ? AND deleted_at IS NULL", googleID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *authRepository) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).Preload("Profile").Where("email = ? AND deleted_at IS NULL", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *authRepository) FindUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).Preload("Profile").Where("id = ? AND deleted_at IS NULL", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *authRepository) FindUserByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).
		Joins("JOIN profiles ON profiles.id = users.id").
		Preload("Profile").
		Where("profiles.username = ? AND users.deleted_at IS NULL", username).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *authRepository) UpdateUser(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *authRepository) CreateToken(ctx context.Context, token *UserToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *authRepository) FindToken(ctx context.Context, token string, tokenType TokenType) (*UserToken, error) {
	var userToken UserToken
	err := r.db.WithContext(ctx).Where("token = ? AND type = ?", token, tokenType).First(&userToken).Error
	if err != nil {
		return nil, err
	}
	return &userToken, nil
}

func (r *authRepository) MarkTokenUsed(ctx context.Context, tokenID uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&UserToken{}).Where("id = ?", tokenID).Update("used_at", &now).Error
}

func (r *authRepository) DeleteToken(ctx context.Context, token string) error {
	return r.db.WithContext(ctx).Where("token = ?", token).Delete(&UserToken{}).Error
}

func (r *authRepository) DeleteUserTokensByType(ctx context.Context, userID uuid.UUID, tokenType TokenType) error {
	return r.db.WithContext(ctx).Where("user_id = ? AND type = ?", userID, tokenType).Delete(&UserToken{}).Error
}

func (r *authRepository) GetLatestUnusedToken(ctx context.Context, userID uuid.UUID, tokenType TokenType) (*UserToken, error) {
	var userToken UserToken
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND type = ? AND used_at IS NULL", userID, tokenType).
		Order("created_at DESC").
		First(&userToken).Error
	if err != nil {
		return nil, err
	}
	return &userToken, nil
}
