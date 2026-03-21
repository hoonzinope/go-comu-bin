package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	notificationsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/notification"
)

type NotificationService = notificationsvc.Service

func NewNotificationService(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, notificationRepository port.NotificationRepository) *NotificationService {
	return notificationsvc.NewService(userRepository, postRepository, commentRepository, notificationRepository)
}
