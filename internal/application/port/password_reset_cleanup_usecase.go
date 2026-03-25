package port

import (
	"context"
	"time"
)

type PasswordResetCleanupUseCase interface {
	CleanupPasswordResetTokens(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error)
}
