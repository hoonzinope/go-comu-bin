package noop

import (
	"context"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.PasswordResetMailSender = (*PasswordResetMailSender)(nil)

type PasswordResetMailSender struct{}

func NewPasswordResetMailSender() *PasswordResetMailSender {
	return &PasswordResetMailSender{}
}

func (s *PasswordResetMailSender) SendPasswordReset(ctx context.Context, email, token string, expiresAt time.Time) error {
	_ = ctx
	_ = email
	_ = token
	_ = expiresAt
	return nil
}
