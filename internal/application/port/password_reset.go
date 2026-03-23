package port

import (
	"context"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type PasswordResetTokenRepository interface {
	Save(ctx context.Context, token *entity.PasswordResetToken) error
	SelectByTokenHash(ctx context.Context, tokenHash string) (*entity.PasswordResetToken, error)
	InvalidateByUser(ctx context.Context, userID int64) error
	Update(ctx context.Context, token *entity.PasswordResetToken) error
}

type PasswordResetTokenIssuer interface {
	Issue() (string, error)
}

type PasswordResetMailSender interface {
	SendPasswordReset(ctx context.Context, email, token string, expiresAt time.Time) error
}
