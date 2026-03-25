package port

import (
	"context"
	"time"
)

type EmailVerificationCleanupUseCase interface {
	CleanupEmailVerificationTokens(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error)
}
