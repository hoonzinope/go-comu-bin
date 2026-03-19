package port

import (
	"context"
	"time"
)

type GuestCleanupUseCase interface {
	CleanupGuests(ctx context.Context, now time.Time, pendingGrace, activeUnusedGrace time.Duration, limit int) (int, error)
}
