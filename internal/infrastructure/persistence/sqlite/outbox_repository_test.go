package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/porttest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutboxRepository_AppendAndFetchReady(t *testing.T) {
	t.Parallel()

	repo := NewOutboxRepository(openTestSQLiteDB(t))
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
	assert.Equal(t, port.OutboxStatusProcessing, messages[0].Status)
	assert.Equal(t, 1, messages[0].AttemptCount)
}

func TestOutboxRepository_MarkRetryAndDead(t *testing.T) {
	t.Parallel()

	repo := NewOutboxRepository(openTestSQLiteDB(t))
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

	_, err := repo.FetchReady(ctx, 1, now)
	require.NoError(t, err)

	next := now.Add(100 * time.Millisecond)
	require.NoError(t, repo.MarkRetry(ctx, "m1", next, "temporary"))
	ready, err := repo.FetchReady(ctx, 1, now.Add(50*time.Millisecond))
	require.NoError(t, err)
	assert.Empty(t, ready)

	ready, err = repo.FetchReady(ctx, 1, now.Add(200*time.Millisecond))
	require.NoError(t, err)
	require.Len(t, ready, 1)
	assert.Equal(t, 2, ready[0].AttemptCount)

	require.NoError(t, repo.MarkDead(ctx, "m1", "permanent"))
	ready, err = repo.FetchReady(ctx, 1, now.Add(time.Second))
	require.NoError(t, err)
	assert.Empty(t, ready)
}

func TestOutboxRepository_MarkSucceededRemovesMessage(t *testing.T) {
	t.Parallel()

	repo := NewOutboxRepository(openTestSQLiteDB(t))
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

	_, err := repo.FetchReady(ctx, 1, now)
	require.NoError(t, err)
	require.NoError(t, repo.MarkSucceeded(ctx, "m1"))

	ready, err := repo.FetchReady(ctx, 1, now.Add(time.Second))
	require.NoError(t, err)
	assert.Empty(t, ready)
}

func TestOutboxRepository_DeadMessageCanBeRequeuedAndDiscarded(t *testing.T) {
	t.Parallel()

	repo := NewOutboxRepository(openTestSQLiteDB(t))
	now := time.Now()
	ctx := context.Background()
	require.NoError(t, repo.Append(ctx, port.OutboxMessage{
		ID:            "dead-1",
		EventName:     "post.changed",
		Payload:       []byte(`{"x":1}`),
		OccurredAt:    now,
		NextAttemptAt: now,
		Status:        port.OutboxStatusPending,
	}))

	ready, err := repo.FetchReady(ctx, 1, now)
	require.NoError(t, err)
	require.Len(t, ready, 1)
	require.NoError(t, repo.MarkDead(ctx, "dead-1", "failed too many times"))

	requeueAt := now.Add(10 * time.Millisecond)
	require.NoError(t, repo.MarkRetry(ctx, "dead-1", requeueAt, "manual retry"))
	ready, err = repo.FetchReady(ctx, 1, now.Add(20*time.Millisecond))
	require.NoError(t, err)
	require.Len(t, ready, 1)
	assert.Equal(t, "dead-1", ready[0].ID)

	require.NoError(t, repo.MarkSucceeded(ctx, "dead-1"))
	ready, err = repo.FetchReady(ctx, 1, now.Add(time.Minute))
	require.NoError(t, err)
	assert.Empty(t, ready)
}

func TestOutboxRepository_SelectDead_WithCursor(t *testing.T) {
	t.Parallel()

	repo := NewOutboxRepository(openTestSQLiteDB(t))
	now := time.Now()
	ctx := context.Background()
	require.NoError(t, repo.Append(ctx,
		port.OutboxMessage{ID: "d1", EventName: "e1", Status: port.OutboxStatusDead, OccurredAt: now, NextAttemptAt: now},
		port.OutboxMessage{ID: "d2", EventName: "e2", Status: port.OutboxStatusDead, OccurredAt: now.Add(time.Second), NextAttemptAt: now},
		port.OutboxMessage{ID: "p1", EventName: "e3", Status: port.OutboxStatusPending, OccurredAt: now, NextAttemptAt: now},
	))

	list, err := repo.SelectDead(ctx, 1, "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "d2", list[0].ID)

	next, err := repo.SelectDead(ctx, 10, "d2")
	require.NoError(t, err)
	require.Len(t, next, 1)
	assert.Equal(t, "d1", next[0].ID)
}

func TestOutboxRepository_SelectByID(t *testing.T) {
	t.Parallel()

	repo := NewOutboxRepository(openTestSQLiteDB(t))
	now := time.Now()
	ctx := context.Background()
	require.NoError(t, repo.Append(ctx,
		port.OutboxMessage{ID: "d1", EventName: "e1", Status: port.OutboxStatusDead, OccurredAt: now, NextAttemptAt: now},
	))

	message, err := repo.SelectByID(ctx, "d1")
	require.NoError(t, err)
	require.NotNil(t, message)
	assert.Equal(t, "d1", message.ID)
	assert.Equal(t, port.OutboxStatusDead, message.Status)

	missing, err := repo.SelectByID(ctx, "missing")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestOutboxRepository_ReclaimsStaleProcessingMessage(t *testing.T) {
	t.Parallel()

	repo := NewOutboxRepository(openTestSQLiteDB(t), WithProcessingTimeout(20*time.Millisecond))
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

	firstBatch, err := repo.FetchReady(ctx, 1, now)
	require.NoError(t, err)
	require.Len(t, firstBatch, 1)
	assert.Equal(t, 1, firstBatch[0].AttemptCount)
	assert.Equal(t, port.OutboxStatusProcessing, firstBatch[0].Status)

	none, err := repo.FetchReady(ctx, 1, now.Add(10*time.Millisecond))
	require.NoError(t, err)
	assert.Empty(t, none)

	reclaimed, err := repo.FetchReady(ctx, 1, now.Add(25*time.Millisecond))
	require.NoError(t, err)
	require.Len(t, reclaimed, 1)
	assert.Equal(t, "m1", reclaimed[0].ID)
	assert.Equal(t, 2, reclaimed[0].AttemptCount)
	assert.Equal(t, port.OutboxStatusProcessing, reclaimed[0].Status)
}

func TestOutboxRepository_UnitOfWorkRollback(t *testing.T) {
	t.Parallel()

	db := openTestSQLiteDB(t)
	outboxRepo := NewOutboxRepository(db)
	uow := NewUnitOfWork(db, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, outboxRepo)

	txErr := errors.New("rollback me")
	err := uow.WithinTransaction(context.Background(), func(tx port.TxScope) error {
		appendErr := tx.Outbox().Append(context.Background(), port.OutboxMessage{
			ID:            "m1",
			EventName:     "post.changed",
			Payload:       []byte(`{"x":1}`),
			OccurredAt:    time.Now(),
			NextAttemptAt: time.Now(),
			Status:        port.OutboxStatusPending,
		})
		require.NoError(t, appendErr)
		return txErr
	})
	require.ErrorIs(t, err, txErr)

	ready, fetchErr := outboxRepo.FetchReady(context.Background(), 10, time.Now().Add(time.Second))
	require.NoError(t, fetchErr)
	assert.Empty(t, ready)
}

func TestOutboxRepository_RespectsCanceledContext(t *testing.T) {
	repo := NewOutboxRepository(openTestSQLiteDB(t))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := repo.Append(ctx, port.OutboxMessage{ID: "m1", EventName: "post.changed"})
	require.ErrorIs(t, err, context.Canceled)
}

func TestSQLiteOutboxRepository_Contract(t *testing.T) {
	t.Parallel()

	porttest.RunOutboxRepositoryContractTests(t, func() port.OutboxStore {
		return NewOutboxRepository(openTestSQLiteDB(t))
	})
}
