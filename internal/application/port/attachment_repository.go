package port

import (
	"context"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type AttachmentRepository interface {
	Save(ctx context.Context, attachment *entity.Attachment) (int64, error)
	SelectByID(ctx context.Context, id int64) (*entity.Attachment, error)
	SelectByUUID(ctx context.Context, attachmentUUID string) (*entity.Attachment, error)
	SelectByPostID(ctx context.Context, postID int64) ([]*entity.Attachment, error)
	SelectCleanupCandidatesBefore(ctx context.Context, cutoff time.Time, limit int) ([]*entity.Attachment, error)
	Update(ctx context.Context, attachment *entity.Attachment) error
	Delete(ctx context.Context, id int64) error
}
