package mail

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/syahrulfahmi/kolaboria-api/config"
)

func TestVerificationTemplate_Rendering(t *testing.T) {
	tmpl, err := template.New("verification").Parse(verificationTemplateHTML)
	if err != nil {
		t.Fatalf("failed to parse verification template: %v", err)
	}

	data := templateData{
		Name:    "<Ayu & Budi>",
		URL:     "https://app.kolaboria.com/verify-email?token=xyz123",
		LogoURL: "https://app.kolaboria.com/logo.png",
		Year:    2026,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("failed to execute verification template: %v", err)
	}

	output := buf.String()

	// Check HTML escaping
	if !strings.Contains(output, "&lt;Ayu &amp; Budi&gt;") {
		t.Errorf("expected HTML escaped name, got output: %s", output)
	}

	// Check logo URL
	if !strings.Contains(output, `src="https://app.kolaboria.com/logo.png"`) {
		t.Errorf("expected public logo URL in img src, got output: %s", output)
	}

	// Check target URL link and fallback
	if !strings.Contains(output, `href="https://app.kolaboria.com/verify-email?token=xyz123"`) {
		t.Errorf("expected verification URL link in output")
	}

	// Check key copy elements
	if !strings.Contains(output, "Verifikasi Email Kamu") {
		t.Errorf("expected title 'Verifikasi Email Kamu'")
	}
	if !strings.Contains(output, "24 jam") {
		t.Errorf("expected 24 hour expiry note")
	}

	// Check footer year
	if !strings.Contains(output, "2026") {
		t.Errorf("expected footer year 2026")
	}
}

func TestResetPasswordTemplate_Rendering(t *testing.T) {
	tmpl, err := template.New("reset_password").Parse(resetPasswordTemplateHTML)
	if err != nil {
		t.Fatalf("failed to parse reset password template: %v", err)
	}

	data := templateData{
		Name:    "Citra",
		URL:     "https://app.kolaboria.com/reset-password?token=reset456",
		LogoURL: "https://app.kolaboria.com/logo.png",
		Year:    2026,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("failed to execute reset password template: %v", err)
	}

	output := buf.String()

	// Check logo URL
	if !strings.Contains(output, `src="https://app.kolaboria.com/logo.png"`) {
		t.Errorf("expected public logo URL in img src")
	}

	// Check key copy elements
	if !strings.Contains(output, "Atur Ulang Kata Sandi") {
		t.Errorf("expected title 'Atur Ulang Kata Sandi'")
	}
	if !strings.Contains(output, "15 menit") {
		t.Errorf("expected 15 minute expiry note")
	}
	if !strings.Contains(output, "Jika kamu tidak meminta pengaturan ulang kata sandi") {
		t.Errorf("expected security notice in reset password email")
	}
}

func TestSMTPMailer_LogoURLFallback(t *testing.T) {
	mailer := NewSMTPMailer(config.MailConfig{}, "https://app.kolaboria.com/")
	sMailer, ok := mailer.(*smtpMailer)
	if !ok {
		t.Fatalf("expected *smtpMailer type")
	}

	if sMailer.logoURL != "https://app.kolaboria.com/logo.png" {
		t.Errorf("expected trimmed public logo URL, got '%s'", sMailer.logoURL)
	}

	// Test empty frontend URL fallback
	emptyMailer := NewSMTPMailer(config.MailConfig{}, "")
	sEmptyMailer := emptyMailer.(*smtpMailer)

	data := sEmptyMailer.newTemplateData("User", "https://app.kolaboria.com")
	if data.LogoURL != "" {
		t.Errorf("expected empty LogoURL when frontendURL is empty, got '%s'", data.LogoURL)
	}

	if data.Year != time.Now().Year() {
		t.Errorf("expected current year %d, got %d", time.Now().Year(), data.Year)
	}

	// Verify template uses the text fallback when no public frontend URL is available.
	tmpl, err := template.New("verification").Parse(verificationTemplateHTML)
	if err != nil {
		t.Fatalf("failed to parse verification template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("failed to execute template: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Kolaboria") || strings.Contains(output, "<img") {
		t.Errorf("expected text logo fallback when frontendURL is empty")
	}
}

func TestSMTPMailer_UsesConfiguredPublicLogoURL(t *testing.T) {
	mailer := NewSMTPMailer(
		config.MailConfig{LogoURL: "https://cdn.example.com/kolaboria-logo.png"},
		"http://localhost:3000",
	)
	sMailer := mailer.(*smtpMailer)

	if sMailer.logoURL != "https://cdn.example.com/kolaboria-logo.png" {
		t.Fatalf("expected configured public logo URL, got %q", sMailer.logoURL)
	}
}

func TestSMTPMailer_SendDevFallback(t *testing.T) {
	mailer := NewSMTPMailer(config.MailConfig{}, "https://app.kolaboria.com")

	err := mailer.SendVerificationEmail("test@example.com", "Test User", "https://app.kolaboria.com/verify")
	if err != nil {
		t.Errorf("expected SendVerificationEmail to succeed in dev fallback, got: %v", err)
	}

	err = mailer.SendResetPasswordEmail("test@example.com", "Test User", "https://app.kolaboria.com/reset")
	if err != nil {
		t.Errorf("expected SendResetPasswordEmail to succeed in dev fallback, got: %v", err)
	}
}
