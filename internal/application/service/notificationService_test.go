package service

import (
	"context"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationService_GetMyNotificationsAndMarkRead(t *testing.T) {
	repositories := newTestRepositories()
	recipientID := seedUser(repositories.user, "alice", "pw", "user")
	actorID := seedUser(repositories.user, "bob", "pw", "user")
	postID := seedPost(repositories.post, actorID, seedBoard(repositories.board, "free", "desc"), "hello", "content")
	commentID := seedComment(repositories.comment, actorID, postID, "reply")

	notificationID, err := repositories.notification.Save(context.Background(), entity.NewNotification(
		recipientID,
		actorID,
		entity.NotificationTypeCommentReplied,
		postID,
		commentID,
		"bob",
		"hello",
		"reply",
	))
	require.NoError(t, err)

	svc := NewNotificationService(repositories.user, repositories.post, repositories.comment, repositories.notification)

	list, err := svc.GetMyNotifications(context.Background(), recipientID, 10, "")
	require.NoError(t, err)
	require.Len(t, list.Notifications, 1)
	assert.Equal(t, model.NotificationTypeCommentReplied, list.Notifications[0].Type)
	assert.Equal(t, "bob", list.Notifications[0].ActorName)

	count, err := svc.GetMyUnreadNotificationCount(context.Background(), recipientID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	require.NoError(t, svc.MarkMyNotificationRead(context.Background(), recipientID, list.Notifications[0].UUID))

	count, err = svc.GetMyUnreadNotificationCount(context.Background(), recipientID)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	stored, err := repositories.notification.SelectByID(context.Background(), notificationID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.NotNil(t, stored.ReadAt)
}

func TestNotificationService_MarkMyNotificationRead_RejectsForeignNotification(t *testing.T) {
	repositories := newTestRepositories()
	ownerID := seedUser(repositories.user, "alice", "pw", "user")
	otherID := seedUser(repositories.user, "bob", "pw", "user")

	notification := entity.NewNotification(ownerID, otherID, entity.NotificationTypeMentioned, 0, 0, "bob", "hello", "mention")
	_, err := repositories.notification.Save(context.Background(), notification)
	require.NoError(t, err)

	svc := NewNotificationService(repositories.user, repositories.post, repositories.comment, repositories.notification)

	err = svc.MarkMyNotificationRead(context.Background(), otherID, notification.UUID)
	require.Error(t, err)
	assert.ErrorIs(t, err, customerror.ErrNotificationNotFound)
}
