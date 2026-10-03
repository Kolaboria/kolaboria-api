package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/syahrulfahmi/kolaboria-api/config"
)

type Claims struct {
	UserID     uuid.UUID `json:"user_id"`
	Email      string    `json:"email"`
	Username   string    `json:"username"`
	SystemRole string    `json:"system_role"`
	jwt.RegisteredClaims
}

type JWTService interface {
	GenerateAccessToken(userID uuid.UUID, email string, username string, systemRole string) (string, error)
	ValidateToken(tokenString string) (*Claims, error)
}

type jwtService struct {
	config config.JWTConfig
}

func NewJWTService(cfg config.JWTConfig) JWTService {
	return &jwtService{
		config: cfg,
	}
}

func (s *jwtService) GenerateAccessToken(userID uuid.UUID, email string, username string, systemRole string) (string, error) {
	claims := Claims{
		UserID:     userID,
		Email:      email,
		Username:   username,
		SystemRole: systemRole,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.config.AccessTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.Secret))
}

func (s *jwtService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.config.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
