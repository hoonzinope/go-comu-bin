package port

import (
	"context"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type EmailVerificationTokenRepository interface {
	Save(ctx context.Context, token *entity.EmailVerificationToken) error
	SelectByTokenHash(ctx context.Context, tokenHash string) (*entity.EmailVerificationToken, error)
	InvalidateByUser(ctx context.Context, userID int64) error
	Update(ctx context.Context, token *entity.EmailVerificationToken) error
	DeleteExpiredOrConsumedBefore(ctx context.Context, cutoff time.Time, limit int) (int, error)
}

type EmailVerificationTokenIssuer interface {
	Issue() (string, error)
}

type EmailVerificationMailSender interface {
	SendEmailVerification(ctx context.Context, email, token string, expiresAt time.Time) error
}
