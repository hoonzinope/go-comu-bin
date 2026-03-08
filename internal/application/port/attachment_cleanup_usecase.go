package port

import (
	"context"
	"time"
)

type AttachmentCleanupUseCase interface {
	CleanupAttachments(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error)
}
