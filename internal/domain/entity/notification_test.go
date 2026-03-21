package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNotificationType(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		value, ok := ParseNotificationType("mentioned")
		require.True(t, ok)
		assert.Equal(t, NotificationTypeMentioned, value)
	})

	t.Run("invalid", func(t *testing.T) {
		_, ok := ParseNotificationType("unknown")
		assert.False(t, ok)
	})
}

func TestNotification_NewUnreadAndMarkReadIdempotent(t *testing.T) {
	notification := NewNotification(
		10,
		20,
		NotificationTypeCommentReplied,
		30,
		40,
		"alice",
		"hello",
		"reply preview",
	)

	require.Nil(t, notification.ReadAt)

	notification.MarkRead()
	firstReadAt := notification.ReadAt
	require.NotNil(t, firstReadAt)

	notification.MarkRead()
	require.NotNil(t, notification.ReadAt)
	assert.Equal(t, *firstReadAt, *notification.ReadAt)
}
