package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutboxAdminService_GetDeadMessages_AdminOnly(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewOutboxAdminService(repositories.user, repositories.outbox, newTestAuthorizationPolicy())
	userID := seedUser(repositories.user, "user", "pw", "user")

	_, err := svc.GetDeadMessages(context.Background(), userID, 10, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}

func TestOutboxAdminService_GetDeadMessages_Requeue_Discard(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewOutboxAdminService(repositories.user, repositories.outbox, newTestAuthorizationPolicy())
	adminID := seedUser(repositories.user, "admin", "pw", "admin")

	now := time.Now()
	require.NoError(t, repositories.outbox.Append(
		port.OutboxMessage{ID: "dead-1", EventName: "post.changed", Status: port.OutboxStatusDead, OccurredAt: now, NextAttemptAt: now, AttemptCount: 5, LastError: "failed"},
		port.OutboxMessage{ID: "dead-2", EventName: "comment.changed", Status: port.OutboxStatusDead, OccurredAt: now.Add(time.Second), NextAttemptAt: now, AttemptCount: 5, LastError: "failed"},
	))

	list, err := svc.GetDeadMessages(context.Background(), adminID, 10, "")
	require.NoError(t, err)
	require.Len(t, list.Messages, 2)
	assert.Equal(t, "dead-2", list.Messages[0].ID)

	require.NoError(t, svc.RequeueDeadMessage(context.Background(), adminID, "dead-2"))
	list, err = svc.GetDeadMessages(context.Background(), adminID, 10, "")
	require.NoError(t, err)
	require.Len(t, list.Messages, 1)
	assert.Equal(t, "dead-1", list.Messages[0].ID)

	require.NoError(t, svc.DiscardDeadMessage(context.Background(), adminID, "dead-1"))
	list, err = svc.GetDeadMessages(context.Background(), adminID, 10, "")
	require.NoError(t, err)
	assert.Empty(t, list.Messages)
}

func TestOutboxAdminService_RequeueDiscard_RejectsNonDeadMessage(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewOutboxAdminService(repositories.user, repositories.outbox, newTestAuthorizationPolicy())
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	now := time.Now()
	require.NoError(t, repositories.outbox.Append(
		port.OutboxMessage{ID: "pending-1", EventName: "post.changed", Status: port.OutboxStatusPending, OccurredAt: now, NextAttemptAt: now},
	))

	err := svc.RequeueDeadMessage(context.Background(), adminID, "pending-1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))

	err = svc.DiscardDeadMessage(context.Background(), adminID, "pending-1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}
