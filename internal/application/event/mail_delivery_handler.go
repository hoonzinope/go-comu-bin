package event

import (
	"context"
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
}

func NewMailDeliveryHandler(
	emailVerificationMailer port.EmailVerificationMailSender,
	passwordResetMailer port.PasswordResetMailSender,
	emailVerificationTokens port.EmailVerificationTokenRepository,
	passwordResetTokens port.PasswordResetTokenRepository,
) *MailDeliveryHandler {
	return &MailDeliveryHandler{
		emailVerificationMailer: emailVerificationMailer,
		passwordResetMailer:     passwordResetMailer,
		emailVerificationTokens: emailVerificationTokens,
		passwordResetTokens:     passwordResetTokens,
	}
}

func (h *MailDeliveryHandler) Handle(ctx context.Context, event port.DomainEvent) error {
	switch e := event.(type) {
	case SignupEmailVerificationRequested:
		return h.sendEmailVerification(ctx, e.UserID, e.Email, e.RawToken, e.TokenHash, e.ExpiresAt)
	case EmailVerificationResendRequested:
		return h.sendEmailVerification(ctx, e.UserID, e.Email, e.RawToken, e.TokenHash, e.ExpiresAt)
	case PasswordResetRequested:
		return h.sendPasswordReset(ctx, e.UserID, e.Email, e.RawToken, e.TokenHash, e.ExpiresAt)
	default:
		return nil
	}
}

func (h *MailDeliveryHandler) sendEmailVerification(ctx context.Context, userID int64, email, rawToken, tokenHash string, expiresAt time.Time) error {
	if h == nil || h.emailVerificationMailer == nil || h.emailVerificationTokens == nil || userID <= 0 || tokenHash == "" {
		return nil
	}
	if err := h.emailVerificationMailer.SendEmailVerification(ctx, email, rawToken, expiresAt); err != nil {
		return customerror.Wrap(customerror.ErrInternalServerError, "send email verification mail", err)
	}
	if err := h.activateEmailVerificationToken(ctx, tokenHash); err != nil {
		return err
	}
	return nil
}

func (h *MailDeliveryHandler) sendPasswordReset(ctx context.Context, userID int64, email, rawToken, tokenHash string, expiresAt time.Time) error {
	if h == nil || h.passwordResetMailer == nil || h.passwordResetTokens == nil || userID <= 0 || tokenHash == "" {
		return nil
	}
	if err := h.passwordResetMailer.SendPasswordReset(ctx, email, rawToken, expiresAt); err != nil {
		return customerror.Wrap(customerror.ErrInternalServerError, "send password reset mail", err)
	}
	if err := h.activatePasswordResetToken(ctx, tokenHash); err != nil {
		return err
	}
	return nil
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
