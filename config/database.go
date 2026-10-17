package config

import "fmt"

type DatabaseConfig struct {
	Host         string `mapstructure:"DB_HOST"`
	Port         string `mapstructure:"DB_PORT"`
	Name         string `mapstructure:"DB_NAME"`
	User         string `mapstructure:"DB_USER"`
	Password     string `mapstructure:"DB_PASSWORD"`
	SSLMode      string `mapstructure:"DB_SSLMODE"`
	TimeZone     string `mapstructure:"DB_TIMEZONE"`
	MaxOpenConns int    `mapstructure:"DB_MAX_OPEN_CONNS"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		d.Host, d.User, d.Password, d.Name, d.Port, d.SSLMode, d.TimeZone)
}
