package port

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
)

type NotificationUseCase interface {
	GetMyNotifications(ctx context.Context, userID int64, limit int, cursor string) (*model.NotificationList, error)
	GetMyUnreadNotificationCount(ctx context.Context, userID int64) (int, error)
	MarkMyNotificationRead(ctx context.Context, userID int64, notificationUUID string) error
}
