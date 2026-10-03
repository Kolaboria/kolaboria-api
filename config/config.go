package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Mail     MailConfig
	Google   GoogleConfig
}

// Load preserves the API's optional .env file and environment-variable behavior.
func Load() (*Config, error) {
	return loadConfig(".env", false, false)
}

// LoadFromFile requires the explicitly selected configuration file to exist.
func LoadFromFile(path string) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("configuration file path is empty; use LoadFromEnvironment for explicit OS environment configuration")
	}
	return loadConfig(path, true, true)
}

// LoadFromEnvironment reads only OS environment variables and requires an explicit database target.
func LoadFromEnvironment() (*Config, error) {
	return loadConfig("", true, true)
}

func loadConfig(path string, requireFile, requireDatabaseTarget bool) (*Config, error) {
	v := viper.New()
	v.SetConfigType("env")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil && (requireFile || !os.IsNotExist(err)) {
			return nil, fmt.Errorf("read configuration: %w", err)
		}
	}

	setDefaults(v)
	var cfg Config
	if err := v.Unmarshal(&cfg.App); err != nil {
		return nil, fmt.Errorf("load app config: %w", err)
	}
	if err := v.Unmarshal(&cfg.Database); err != nil {
		return nil, fmt.Errorf("load database config: %w", err)
	}
	if err := v.Unmarshal(&cfg.JWT); err != nil {
		return nil, fmt.Errorf("load JWT config: %w", err)
	}
	if err := v.Unmarshal(&cfg.Mail); err != nil {
		return nil, fmt.Errorf("load mail config: %w", err)
	}
	if err := v.Unmarshal(&cfg.Google); err != nil {
		return nil, fmt.Errorf("load Google config: %w", err)
	}
	if origins := strings.TrimSpace(v.GetString("APP_ALLOW_ORIGINS")); origins != "" {
		for _, origin := range strings.Split(origins, ",") {
			cfg.App.AllowOrigins = append(cfg.App.AllowOrigins, strings.TrimSpace(origin))
		}
	} else {
		cfg.App.AllowOrigins = []string{}
	}
	if requireDatabaseTarget {
		for _, field := range []struct{ key, value string }{
			{"DB_HOST", cfg.Database.Host},
			{"DB_PORT", cfg.Database.Port},
			{"DB_NAME", cfg.Database.Name},
			{"DB_USER", cfg.Database.User},
		} {
			environmentValue, fromEnvironment := os.LookupEnv(field.key)
			explicitValue := (fromEnvironment && strings.TrimSpace(environmentValue) != "") || v.InConfig(field.key)
			if !explicitValue || strings.TrimSpace(field.value) == "" {
				return nil, fmt.Errorf("%s must be explicitly configured", field.key)
			}
		}
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	defaults := map[string]any{
		"APP_NAME": "kolaboria-api", "APP_ENV": "development", "APP_PORT": "8080", "APP_DEBUG": true,
		"APP_FRONTEND_URL": "http://localhost:3000",
		"DB_HOST":          "localhost", "DB_PORT": "5432", "DB_NAME": "kolaboria", "DB_USER": "postgres",
		"DB_PASSWORD": "postgres", "DB_SSLMODE": "disable", "DB_TIMEZONE": "Asia/Jakarta",
		"JWT_SECRET": "change-this-to-a-long-random-secret", "JWT_ACCESS_EXPIRY": "15m", "JWT_REFRESH_EXPIRY": "168h",
		"MAIL_HOST": "smtp.mailtrap.io", "MAIL_PORT": 587, "MAIL_USERNAME": "", "MAIL_PASSWORD": "",
		"MAIL_FROM_NAME": "Kolaboria", "MAIL_FROM_EMAIL": "noreply@kolaboria.com", "MAIL_LOGO_URL": "",
		"GOOGLE_CLIENT_ID": "", "GOOGLE_CLIENT_SECRET": "", "GOOGLE_REDIRECT_URI": "http://localhost:3000/auth/callback",
	}
	for key, value := range defaults {
		v.SetDefault(key, value)
	}
}
