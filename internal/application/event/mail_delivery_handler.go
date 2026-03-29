package event

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
)

var _ port.EventHandler = (*MailDeliveryHandler)(nil)

type MailDeliveryHandler struct {
	emailVerificationMailer port.EmailVerificationMailSender
	passwordResetMailer     port.PasswordResetMailSender
	emailVerificationTokens port.EmailVerificationTokenRepository
	passwordResetTokens     port.PasswordResetTokenRepository
	capturePath             string
	mu                      sync.Mutex
}

func NewMailDeliveryHandler(
	emailVerificationMailer port.EmailVerificationMailSender,
	passwordResetMailer port.PasswordResetMailSender,
	emailVerificationTokens port.EmailVerificationTokenRepository,
	passwordResetTokens port.PasswordResetTokenRepository,
	capturePath string,
) *MailDeliveryHandler {
	return &MailDeliveryHandler{
		emailVerificationMailer: emailVerificationMailer,
		passwordResetMailer:     passwordResetMailer,
		emailVerificationTokens: emailVerificationTokens,
		passwordResetTokens:     passwordResetTokens,
		capturePath:             strings.TrimSpace(capturePath),
	}
}

func (h *MailDeliveryHandler) Handle(ctx context.Context, event port.DomainEvent) error {
	switch e := event.(type) {
	case SignupEmailVerificationRequested:
		return h.sendEmailVerification(ctx, "email.verification.signup.requested", e.UserID, e.Email, e.RawToken, e.TokenHash, e.ExpiresAt)
	case EmailVerificationResendRequested:
		return h.sendEmailVerification(ctx, "email.verification.resend.requested", e.UserID, e.Email, e.RawToken, e.TokenHash, e.ExpiresAt)
	case PasswordResetRequested:
		return h.sendPasswordReset(ctx, "password.reset.requested", e.UserID, e.Email, e.RawToken, e.TokenHash, e.ExpiresAt)
	default:
		return nil
	}
}

func (h *MailDeliveryHandler) sendEmailVerification(ctx context.Context, eventName string, userID int64, email, rawToken, tokenHash string, expiresAt time.Time) error {
	if h == nil || h.emailVerificationMailer == nil || h.emailVerificationTokens == nil || userID <= 0 || tokenHash == "" {
		return nil
	}
	if err := h.emailVerificationMailer.SendEmailVerification(ctx, email, rawToken, expiresAt); err != nil {
		return customerror.WrapMailDelivery("send email verification mail", err)
	}
	h.captureToken(eventName, userID, email, rawToken, expiresAt)
	if err := h.activateEmailVerificationToken(ctx, tokenHash); err != nil {
		return err
	}
	return nil
}

func (h *MailDeliveryHandler) sendPasswordReset(ctx context.Context, eventName string, userID int64, email, rawToken, tokenHash string, expiresAt time.Time) error {
	if h == nil || h.passwordResetMailer == nil || h.passwordResetTokens == nil || userID <= 0 || tokenHash == "" {
		return nil
	}
	if err := h.passwordResetMailer.SendPasswordReset(ctx, email, rawToken, expiresAt); err != nil {
		return customerror.WrapMailDelivery("send password reset mail", err)
	}
	h.captureToken(eventName, userID, email, rawToken, expiresAt)
	if err := h.activatePasswordResetToken(ctx, tokenHash); err != nil {
		return err
	}
	return nil
}

func (h *MailDeliveryHandler) captureToken(eventName string, userID int64, email, rawToken string, expiresAt time.Time) {
	if h == nil || strings.TrimSpace(h.capturePath) == "" || strings.TrimSpace(rawToken) == "" {
		return
	}
	record := struct {
		EventName string    `json:"event_name"`
		UserID    int64     `json:"user_id"`
		Email     string    `json:"email"`
		RawToken  string    `json:"raw_token"`
		ExpiresAt time.Time `json:"expires_at"`
		CreatedAt time.Time `json:"created_at"`
	}{
		EventName: eventName,
		UserID:    userID,
		Email:     email,
		RawToken:  rawToken,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(h.capturePath), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(h.capturePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(payload, '\n'))
}

func (h *MailDeliveryHandler) activateEmailVerificationToken(ctx context.Context, tokenHash string) error {
	now := time.Now()
	token, err := h.emailVerificationTokens.SelectByTokenHash(ctx, tokenHash)
	if err != nil {
		return customerror.WrapRepository("select email verification token after mail send", err)
	}
	if token == nil || !token.IsConsumed() || token.IsExpired(now) {
		return nil
	}
	token.ConsumedAt = nil
	if err := h.emailVerificationTokens.Update(ctx, token); err != nil {
		return customerror.WrapRepository("activate email verification token after mail send", err)
	}
	return nil
}

func (h *MailDeliveryHandler) activatePasswordResetToken(ctx context.Context, tokenHash string) error {
	now := time.Now()
	token, err := h.passwordResetTokens.SelectByTokenHash(ctx, tokenHash)
	if err != nil {
		return customerror.WrapRepository("select password reset token after mail send", err)
	}
	if token == nil || !token.IsConsumed() || token.IsExpired(now) {
		return nil
	}
	token.ConsumedAt = nil
	if err := h.passwordResetTokens.Update(ctx, token); err != nil {
		return customerror.WrapRepository("activate password reset token after mail send", err)
	}
	return nil
}
