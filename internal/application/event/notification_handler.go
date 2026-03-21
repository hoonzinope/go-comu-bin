package event

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.EventHandler = (*NotificationHandler)(nil)

type NotificationHandler struct {
	notificationRepository port.NotificationRepository
}

func NewNotificationHandler(notificationRepository port.NotificationRepository) *NotificationHandler {
	return &NotificationHandler{notificationRepository: notificationRepository}
}

func (h *NotificationHandler) Handle(ctx context.Context, event port.DomainEvent) error {
	triggered, ok := event.(NotificationTriggered)
	if !ok {
		return nil
	}
	notification := entity.NewNotification(
		triggered.RecipientUserID,
		triggered.ActorUserID,
		triggered.Type,
		triggered.PostID,
		triggered.CommentID,
		triggered.ActorNameSnapshot,
		triggered.PostTitleSnapshot,
		triggered.CommentPreviewSnapshot,
	)
	notification.CreatedAt = triggered.At
	notification.DedupKey = triggered.EventID
	if _, err := h.notificationRepository.Save(ctx, notification); err != nil {
		return customerror.WrapRepository("save notification from event", err)
	}
	return nil
}
