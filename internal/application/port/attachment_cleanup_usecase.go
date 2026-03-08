package port

import (
	"context"
	"time"
)

type AttachmentCleanupUseCase interface {
	CleanupOrphanAttachments(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error)
}
