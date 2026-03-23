package smtp

import (
	netsmtp "net/smtp"
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
	assert.Contains(t, sentMsg, "verify-token")
}
