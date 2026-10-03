package config

type MailConfig struct {
	Host      string `mapstructure:"MAIL_HOST"`
	Port      int    `mapstructure:"MAIL_PORT"`
	Username  string `mapstructure:"MAIL_USERNAME"`
	Password  string `mapstructure:"MAIL_PASSWORD"`
	FromName  string `mapstructure:"MAIL_FROM_NAME"`
	FromEmail string `mapstructure:"MAIL_FROM_EMAIL"`
	LogoURL   string `mapstructure:"MAIL_LOGO_URL"`
}
