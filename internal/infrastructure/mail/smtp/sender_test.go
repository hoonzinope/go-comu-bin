package smtp

import (
	netsmtp "net/smtp"
	"net/url"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSender_BuildMessage(t *testing.T) {
	cfg := config.Config{}
	cfg.Delivery.Mail.SMTP.Host = "smtp.example.com"
	cfg.Delivery.Mail.SMTP.Port = 587
	cfg.Delivery.Mail.SMTP.From = "noreply@example.com"

	sender := NewSender(cfg)
	msg, err := sender.buildMessage("alice@example.com", "Email verification", "Use this token:\n\nverify-token\n")
	require.NoError(t, err)

	body := string(msg)
	assert.Contains(t, body, "From: noreply@example.com")
	assert.Contains(t, body, "To: alice@example.com")
	assert.Contains(t, body, "Subject: Email verification")
	assert.Contains(t, body, "verify-token")
}

func TestSender_SendEmailVerification_UsesConfiguredTransport(t *testing.T) {
	cfg := config.Config{}
	cfg.Delivery.Mail.SMTP.Host = "smtp.example.com"
	cfg.Delivery.Mail.SMTP.Port = 587
	cfg.Delivery.Mail.SMTP.From = "noreply@example.com"
	cfg.Delivery.Mail.EmailVerification.BaseURL = "https://app.example.com/verify-email"

	sender := NewSender(cfg)
	var sentAddr string
	var sentMsg string
	sender.sendMail = func(addr string, auth netsmtp.Auth, from string, to []string, msg []byte) error {
		_ = auth
		_ = from
		_ = to
		sentAddr = addr
		sentMsg = string(msg)
		return nil
	}
	require.NoError(t, sender.SendEmailVerification(t.Context(), "alice@example.com", "verify-token", time.Now().Add(time.Hour)))
	assert.Equal(t, "smtp.example.com:587", sentAddr)
	assert.Contains(t, sentMsg, "Subject: Email verification")
	assert.Contains(t, sentMsg, "https://app.example.com/verify-email?token="+url.QueryEscape("verify-token"))
	assert.Contains(t, sentMsg, "verify-token")
}

func TestSender_SendPasswordReset_IncludesFrontendLinkAndFallbackToken(t *testing.T) {
	cfg := config.Config{}
	cfg.Delivery.Mail.SMTP.Host = "smtp.example.com"
	cfg.Delivery.Mail.SMTP.Port = 587
	cfg.Delivery.Mail.SMTP.From = "noreply@example.com"
	cfg.Delivery.Mail.PasswordReset.BaseURL = "https://app.example.com/reset-password"

	sender := NewSender(cfg)
	var sentMsg string
	sender.sendMail = func(addr string, auth netsmtp.Auth, from string, to []string, msg []byte) error {
		_ = addr
		_ = auth
		_ = from
		_ = to
		sentMsg = string(msg)
		return nil
	}

	require.NoError(t, sender.SendPasswordReset(t.Context(), "alice@example.com", "reset-token+/=", time.Now().Add(time.Hour)))
	assert.Contains(t, sentMsg, "Subject: Password reset")
	assert.Contains(t, sentMsg, "https://app.example.com/reset-password?token="+url.QueryEscape("reset-token+/="))
	assert.Contains(t, sentMsg, "reset-token+/=")
}
