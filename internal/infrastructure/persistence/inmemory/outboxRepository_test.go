package inmemory

import (
	"errors"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutboxRepository_AppendAndFetchReady(t *testing.T) {
	repo := NewOutboxRepository()
	now := time.Now()
	require.NoError(t, repo.Append(port.OutboxMessage{
		ID:            "m1",
		EventName:     "post.changed",
		Payload:       []byte(`{"x":1}`),
		OccurredAt:    now,
		NextAttemptAt: now,
		Status:        port.OutboxStatusPending,
	}))

	messages, err := repo.FetchReady(10, now)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "m1", messages[0].ID)
	assert.Equal(t, port.OutboxStatusProcessing, messages[0].Status)
	assert.Equal(t, 1, messages[0].AttemptCount)
}

func TestOutboxRepository_MarkRetryAndDead(t *testing.T) {
	repo := NewOutboxRepository()
	now := time.Now()
	require.NoError(t, repo.Append(port.OutboxMessage{
		ID:            "m1",
		EventName:     "post.changed",
		Payload:       []byte(`{"x":1}`),
		OccurredAt:    now,
		NextAttemptAt: now,
		Status:        port.OutboxStatusPending,
	}))
	_, err := repo.FetchReady(1, now)
	require.NoError(t, err)

	next := now.Add(100 * time.Millisecond)
	require.NoError(t, repo.MarkRetry("m1", next, "temporary"))
	ready, err := repo.FetchReady(1, now.Add(50*time.Millisecond))
	require.NoError(t, err)
	assert.Empty(t, ready)

	ready, err = repo.FetchReady(1, now.Add(200*time.Millisecond))
	require.NoError(t, err)
	require.Len(t, ready, 1)
	assert.Equal(t, 2, ready[0].AttemptCount)

	require.NoError(t, repo.MarkDead("m1", "permanent"))
	ready, err = repo.FetchReady(1, now.Add(time.Second))
	require.NoError(t, err)
	assert.Empty(t, ready)
}

func TestOutboxRepository_MarkSucceededRemovesMessage(t *testing.T) {
	repo := NewOutboxRepository()
	now := time.Now()
	require.NoError(t, repo.Append(port.OutboxMessage{
		ID:            "m1",
		EventName:     "post.changed",
		Payload:       []byte(`{"x":1}`),
		OccurredAt:    now,
		NextAttemptAt: now,
		Status:        port.OutboxStatusPending,
	}))

	_, err := repo.FetchReady(1, now)
	require.NoError(t, err)
	require.NoError(t, repo.MarkSucceeded("m1"))

	ready, err := repo.FetchReady(1, now.Add(time.Second))
	require.NoError(t, err)
	assert.Empty(t, ready)
}

func TestOutboxRepository_ReclaimsStaleProcessingMessage(t *testing.T) {
	repo := NewOutboxRepository(WithProcessingTimeout(20 * time.Millisecond))
	now := time.Now()
	require.NoError(t, repo.Append(port.OutboxMessage{
		ID:            "m1",
		EventName:     "post.changed",
		Payload:       []byte(`{"x":1}`),
		OccurredAt:    now,
		NextAttemptAt: now,
		Status:        port.OutboxStatusPending,
	}))

	firstBatch, err := repo.FetchReady(1, now)
	require.NoError(t, err)
	require.Len(t, firstBatch, 1)
	assert.Equal(t, 1, firstBatch[0].AttemptCount)
	assert.Equal(t, port.OutboxStatusProcessing, firstBatch[0].Status)

	// 아직 lease timeout 이내이므로 재수거되지 않는다.
	none, err := repo.FetchReady(1, now.Add(10*time.Millisecond))
	require.NoError(t, err)
	assert.Empty(t, none)

	// lease timeout 이후에는 stale processing을 reclaim 후 재수거한다.
	reclaimed, err := repo.FetchReady(1, now.Add(25*time.Millisecond))
	require.NoError(t, err)
	require.Len(t, reclaimed, 1)
	assert.Equal(t, "m1", reclaimed[0].ID)
	assert.Equal(t, 2, reclaimed[0].AttemptCount)
	assert.Equal(t, port.OutboxStatusProcessing, reclaimed[0].Status)
}

func TestOutboxRepository_DeadMessageCanBeRequeuedAndDiscarded(t *testing.T) {
	repo := NewOutboxRepository()
	now := time.Now()
	require.NoError(t, repo.Append(port.OutboxMessage{
		ID:            "dead-1",
		EventName:     "post.changed",
		Payload:       []byte(`{"x":1}`),
		OccurredAt:    now,
		NextAttemptAt: now,
		Status:        port.OutboxStatusPending,
	}))

	ready, err := repo.FetchReady(1, now)
	require.NoError(t, err)
	require.Len(t, ready, 1)
	require.NoError(t, repo.MarkDead("dead-1", "failed too many times"))

	// dead -> pending 재처리(운영자 수동 조치) 경로
	requeueAt := now.Add(10 * time.Millisecond)
	require.NoError(t, repo.MarkRetry("dead-1", requeueAt, "manual retry"))
	ready, err = repo.FetchReady(1, now.Add(20*time.Millisecond))
	require.NoError(t, err)
	require.Len(t, ready, 1)
	assert.Equal(t, "dead-1", ready[0].ID)

	// discard(영구 제거) 경로
	require.NoError(t, repo.MarkSucceeded("dead-1"))
	ready, err = repo.FetchReady(1, now.Add(time.Minute))
	require.NoError(t, err)
	assert.Empty(t, ready)
}

func TestUnitOfWork_OutboxAppendRollback(t *testing.T) {
	userRepository := NewUserRepository()
	boardRepository := NewBoardRepository()
	tagRepository := NewTagRepository()
	postTagRepository := NewPostTagRepository()
	postRepository := NewPostRepository(tagRepository, postTagRepository)
	commentRepository := NewCommentRepository()
	reactionRepository := NewReactionRepository()
	attachmentRepository := NewAttachmentRepository()
	outboxRepository := NewOutboxRepository()
	unitOfWork := NewUnitOfWork(
		userRepository,
		boardRepository,
		postRepository,
		tagRepository,
		postTagRepository,
		commentRepository,
		reactionRepository,
		attachmentRepository,
		outboxRepository,
	)

	txErr := errors.New("rollback me")
	err := unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		appendErr := tx.Outbox().Append(port.OutboxMessage{
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

	ready, fetchErr := outboxRepository.FetchReady(10, time.Now().Add(time.Second))
	require.NoError(t, fetchErr)
	assert.Empty(t, ready)
}
