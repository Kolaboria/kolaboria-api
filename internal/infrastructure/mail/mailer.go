package mail

import (
	"bytes"
	"crypto/tls"
	_ "embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/smtp"
	"strings"
	"time"

	"github.com/syahrulfahmi/kolaboria-api/config"
)

//go:embed templates/verification.html
var verificationTemplateHTML string

//go:embed templates/reset_password.html
var resetPasswordTemplateHTML string

type Mailer interface {
	SendVerificationEmail(to, name, verificationURL string) error
	SendResetPasswordEmail(to, name, resetURL string) error
}

type templateData struct {
	Name    string
	URL     string
	LogoURL string
	Year    int
}

type smtpMailer struct {
	config  config.MailConfig
	logoURL string
}

func NewSMTPMailer(cfg config.MailConfig, frontendURL string) Mailer {
	logoURL := strings.TrimSpace(cfg.LogoURL)
	if logoURL == "" && frontendURL != "" {
		logoURL = strings.TrimRight(frontendURL, "/") + "/logo.png"
	}
	return &smtpMailer{
		config:  cfg,
		logoURL: logoURL,
	}
}

func (m *smtpMailer) newTemplateData(name, targetURL string) templateData {
	return templateData{
		Name:    name,
		URL:     targetURL,
		LogoURL: m.logoURL,
		Year:    time.Now().Year(),
	}
}

func (m *smtpMailer) SendVerificationEmail(to, name, verificationURL string) error {
	tmpl, err := template.New("verification").Parse(verificationTemplateHTML)
	if err != nil {
		return fmt.Errorf("failed to parse verification template: %w", err)
	}

	var body bytes.Buffer
	data := m.newTemplateData(name, verificationURL)

	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to execute verification template: %w", err)
	}

	return m.sendHTML(to, "Verifikasi Email Anda - Kolaboria", body.String())
}

func (m *smtpMailer) SendResetPasswordEmail(to, name, resetURL string) error {
	tmpl, err := template.New("reset_password").Parse(resetPasswordTemplateHTML)
	if err != nil {
		return fmt.Errorf("failed to parse reset password template: %w", err)
	}

	var body bytes.Buffer
	data := m.newTemplateData(name, resetURL)

	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to execute reset password template: %w", err)
	}

	return m.sendHTML(to, "Atur Ulang Kata Sandi - Kolaboria", body.String())
}

func (m *smtpMailer) sendHTML(to, subject, htmlBody string) error {
	// Fallback to logging in development if Mail config is not provided
	if m.config.Host == "" || m.config.Username == "" {
		slog.Info("SMTP configuration missing or empty, logging email contents (Development Fallback)",
			"to", to,
			"subject", subject,
			"body_length", len(htmlBody),
		)
		// Print the email URL or contents nicely to standard output so developers can copy the links
		fmt.Printf("\n=== [DEVELOPMENT EMAIL LOG] ===\nTo: %s\nSubject: %s\nBody snippet (usually contains token link):\n%s\n=================================\n\n", to, subject, htmlBody)
		return nil
	}

	fromHeader := fmt.Sprintf("%s <%s>", m.config.FromName, m.config.FromEmail)
	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s", fromHeader, to, subject, htmlBody)

	auth := smtp.PlainAuth("", m.config.Username, m.config.Password, m.config.Host)
	addr := fmt.Sprintf("%s:%d", m.config.Host, m.config.Port)
	var err error

	if m.config.Port == 465 {
		// SMTP over SSL/TLS (SMTPS)
		tlsConfig := &tls.Config{
			ServerName: m.config.Host,
		}

		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			slog.Error("failed to connect via SSL/TLS", "to", to, "error", err)
			return fmt.Errorf("failed to connect via SSL/TLS: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, m.config.Host)
		if err != nil {
			slog.Error("failed to create SMTP client", "to", to, "error", err)
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}
		defer client.Close()

		if err = client.Auth(auth); err != nil {
			slog.Error("SMTP auth failed", "to", to, "error", err)
			return fmt.Errorf("SMTP auth failed: %w", err)
		}

		if err = client.Mail(m.config.FromEmail); err != nil {
			slog.Error("SMTP Mail command failed", "to", to, "error", err)
			return fmt.Errorf("SMTP Mail command failed: %w", err)
		}

		if err = client.Rcpt(to); err != nil {
			slog.Error("SMTP Rcpt command failed", "to", to, "error", err)
			return fmt.Errorf("SMTP Rcpt command failed: %w", err)
		}

		w, err := client.Data()
		if err != nil {
			slog.Error("SMTP Data command failed", "to", to, "error", err)
			return fmt.Errorf("SMTP Data command failed: %w", err)
		}

		_, err = w.Write([]byte(msg))
		if err != nil {
			slog.Error("failed to write email body", "to", to, "error", err)
			return fmt.Errorf("failed to write email body: %w", err)
		}

		err = w.Close()
		if err != nil {
			slog.Error("failed to close email body writer", "to", to, "error", err)
			return fmt.Errorf("failed to close email body writer: %w", err)
		}

		err = client.Quit()
		if err != nil {
			slog.Error("failed to quit SMTP client", "to", to, "error", err)
		}
	} else {
		// Standard smtp.SendMail (uses STARTTLS if supported by server)
		err = smtp.SendMail(addr, auth, m.config.FromEmail, []string{to}, []byte(msg))
		if err != nil {
			slog.Error("failed to send email via SMTP", "to", to, "error", err)
			return fmt.Errorf("failed to send email: %w", err)
		}
	}

	slog.Info("email sent successfully", "to", to, "subject", subject)
	return nil
}
