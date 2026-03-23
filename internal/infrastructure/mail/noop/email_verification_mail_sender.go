package noop

import (
	"context"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.EmailVerificationMailSender = (*EmailVerificationMailSender)(nil)

type EmailVerificationMailSender struct{}

func NewEmailVerificationMailSender() *EmailVerificationMailSender {
	return &EmailVerificationMailSender{}
}

func (s *EmailVerificationMailSender) SendEmailVerification(ctx context.Context, email, token string, expiresAt time.Time) error {
	_ = ctx
	_ = email
	_ = token
	_ = expiresAt
	return nil
}
