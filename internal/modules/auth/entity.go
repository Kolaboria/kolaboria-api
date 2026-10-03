package auth

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TokenType string

const (
	TokenTypeRefresh           TokenType = "refresh"
	TokenTypeEmailVerification TokenType = "email_verification"
	TokenTypePasswordReset     TokenType = "password_reset"
)

type User struct {
	ID              uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Email           string         `gorm:"type:varchar(255);not null;uniqueIndex"`
	PasswordHash    string         `gorm:"type:varchar(255);not null"`
	EmailVerifiedAt *time.Time     `gorm:"type:timestamptz"`
	IsActive        bool           `gorm:"type:boolean;not null;default:true"`
	GoogleID        *string        `gorm:"type:varchar(255);uniqueIndex"`
	Provider        string         `gorm:"type:varchar(20);not null;default:'email'"`
	CreatedAt       time.Time      `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt       time.Time      `gorm:"type:timestamptz;not null;default:now()"`
	DeletedAt       gorm.DeletedAt `gorm:"index"`

	// Relationship
	Profile *Profile `gorm:"foreignKey:ID"`
}

func (User) TableName() string {
	return "users"
}

type Profile struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	Username      string    `gorm:"type:varchar(100);not null;uniqueIndex"`
	FullName      string    `gorm:"type:varchar(255)"`
	Headline      *string   `gorm:"type:text"`
	Bio           *string   `gorm:"type:text"`
	Avatar        *string   `gorm:"type:text"`
	Location      *string   `gorm:"type:varchar(255)"`
	AccountType   *string   `gorm:"type:varchar(20)"`
	ExternalLinks string    `gorm:"type:jsonb;not null;default:'{}'"`
	IsVerified    bool      `gorm:"type:boolean;not null;default:false"`
	CreatedAt     time.Time `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt     time.Time `gorm:"type:timestamptz;not null;default:now()"`
}

func (Profile) TableName() string {
	return "profiles"
}

func (u *User) GetFullName() string {
	if u != nil && u.Profile != nil {
		return u.Profile.FullName
	}
	return ""
}

func (u *User) GetUsername() string {
	if u != nil && u.Profile != nil {
		return u.Profile.Username
	}
	return ""
}

type UserToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null"`
	Token     string     `gorm:"type:text;not null;uniqueIndex"`
	Type      TokenType  `gorm:"type:token_type;not null"`
	ExpiresAt time.Time  `gorm:"type:timestamptz;not null"`
	UsedAt    *time.Time `gorm:"type:timestamptz"`
	CreatedAt time.Time  `gorm:"type:timestamptz;not null;default:now()"`
}

func (UserToken) TableName() string {
	return "user_tokens"
}
