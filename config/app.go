package config

type AppConfig struct {
	Name         string   `mapstructure:"APP_NAME"`
	Env          string   `mapstructure:"APP_ENV"`
	Port         string   `mapstructure:"APP_PORT"`
	Debug        bool     `mapstructure:"APP_DEBUG"`
	AllowOrigins []string `mapstructure:"APP_ALLOW_ORIGINS"`
	FrontendURL  string   `mapstructure:"APP_FRONTEND_URL"`
}
