package inmemory

import (
	"context"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationRepository_SelectByRecipientUnreadAndMarkRead(t *testing.T) {
	repo := NewNotificationRepository()

	first := entity.NewNotification(100, 200, entity.NotificationTypePostCommented, 300, 0, "bob", "post-a", "comment-a")
	second := entity.NewNotification(100, 201, entity.NotificationTypeMentioned, 301, 401, "carol", "post-b", "comment-b")
	other := entity.NewNotification(101, 202, entity.NotificationTypeMentioned, 302, 402, "dave", "post-c", "comment-c")

	_, err := repo.Save(context.Background(), first)
	require.NoError(t, err)
	_, err = repo.Save(context.Background(), second)
	require.NoError(t, err)
	_, err = repo.Save(context.Background(), other)
	require.NoError(t, err)

	items, err := repo.SelectByRecipientUserID(context.Background(), 100, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, second.UUID, items[0].UUID)
	assert.Equal(t, first.UUID, items[1].UUID)

	count, err := repo.CountUnreadByRecipientUserID(context.Background(), 100)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	require.NoError(t, repo.MarkRead(context.Background(), second.ID))
	require.NoError(t, repo.MarkRead(context.Background(), second.ID))

	count, err = repo.CountUnreadByRecipientUserID(context.Background(), 100)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestNotificationRepository_Save_DeduplicatesByEventID(t *testing.T) {
	repo := NewNotificationRepository()

	first := entity.NewNotification(100, 200, entity.NotificationTypeMentioned, 300, 400, "bob", "post", "preview")
	first.DedupKey = "event-1"
	second := entity.NewNotification(100, 200, entity.NotificationTypeMentioned, 300, 400, "bob", "post", "preview")
	second.DedupKey = "event-1"

	firstID, err := repo.Save(context.Background(), first)
	require.NoError(t, err)
	secondID, err := repo.Save(context.Background(), second)
	require.NoError(t, err)

	assert.Equal(t, firstID, secondID)

	items, err := repo.SelectByRecipientUserID(context.Background(), 100, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
}
