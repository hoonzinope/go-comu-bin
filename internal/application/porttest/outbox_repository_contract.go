package porttest

import (
	"context"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func RunOutboxRepositoryContractTests(t *testing.T, newRepository func() port.OutboxStore) {
	t.Helper()

	t.Run("append fetch and mark succeeded", func(t *testing.T) {
		repo := newRepository()
		now := time.Now()
		ctx := context.Background()
		require.NoError(t, repo.Append(ctx, port.OutboxMessage{
			ID:            "m1",
			EventName:     "post.changed",
			Payload:       []byte(`{"x":1}`),
			OccurredAt:    now,
			NextAttemptAt: now,
			Status:        port.OutboxStatusPending,
		}))

		messages, err := repo.FetchReady(ctx, 10, now)
		require.NoError(t, err)
		require.Len(t, messages, 1)
		assert.Equal(t, "m1", messages[0].ID)
		require.NoError(t, repo.MarkSucceeded(ctx, "m1"))
		ready, err := repo.FetchReady(ctx, 10, now.Add(time.Second))
		require.NoError(t, err)
		assert.Empty(t, ready)
	})

	t.Run("dead cursor and requeue", func(t *testing.T) {
		repo := newRepository()
		now := time.Now()
		ctx := context.Background()
		require.NoError(t, repo.Append(ctx,
			port.OutboxMessage{ID: "d1", EventName: "e1", Status: port.OutboxStatusDead, OccurredAt: now, NextAttemptAt: now},
			port.OutboxMessage{ID: "d2", EventName: "e2", Status: port.OutboxStatusDead, OccurredAt: now.Add(time.Second), NextAttemptAt: now},
		))

		list, err := repo.SelectDead(ctx, 1, "")
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, "d2", list[0].ID)

		message, err := repo.SelectByID(ctx, "d1")
		require.NoError(t, err)
		require.NotNil(t, message)
		assert.Equal(t, port.OutboxStatusDead, message.Status)

		require.NoError(t, repo.MarkRetry(ctx, "d2", now.Add(time.Second), "manual retry"))
		ready, err := repo.FetchReady(ctx, 1, now.Add(2*time.Second))
		require.NoError(t, err)
		require.Len(t, ready, 1)
		assert.Equal(t, "d2", ready[0].ID)
	})

	t.Run("requeue rejected for missing message", func(t *testing.T) {
		repo := newRepository()
		ctx := context.Background()
		require.NoError(t, repo.MarkRetry(ctx, "missing", time.Now(), "retry"))
		require.NoError(t, repo.MarkDead(ctx, "missing", "dead"))
	})
}
