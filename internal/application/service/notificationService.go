package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	notificationsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/notification"
)

func NewNotificationService(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, notificationRepository port.NotificationRepository) *notificationsvc.Service {
	return notificationsvc.NewService(userRepository, postRepository, commentRepository, notificationRepository)
}
