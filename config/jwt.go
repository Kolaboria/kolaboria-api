package config

import "time"

type JWTConfig struct {
	Secret             string        `mapstructure:"JWT_SECRET"`
	AccessTokenExpiry  time.Duration `mapstructure:"JWT_ACCESS_EXPIRY"`
	RefreshTokenExpiry time.Duration `mapstructure:"JWT_REFRESH_EXPIRY"`
}
