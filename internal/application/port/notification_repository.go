package port

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type NotificationRepository interface {
	Save(ctx context.Context, notification *entity.Notification) (int64, error)
	SelectByID(ctx context.Context, id int64) (*entity.Notification, error)
	SelectByUUID(ctx context.Context, notificationUUID string) (*entity.Notification, error)
	SelectByRecipientUserID(ctx context.Context, recipientUserID int64, limit int, lastID int64) ([]*entity.Notification, error)
	CountUnreadByRecipientUserID(ctx context.Context, recipientUserID int64) (int, error)
	MarkRead(ctx context.Context, id int64) error
}
