package service

import (
	"context"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
	"testing"
	"time"

	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type spyOutboxAppender struct {
	messages []port.OutboxMessage
}

func (s *spyOutboxAppender) Append(ctx context.Context, messages ...port.OutboxMessage) error {
	_ = ctx
	s.messages = append(s.messages, messages...)
	return nil
}

type spyActionDispatcher struct {
	events []port.DomainEvent
}

func (s *spyActionDispatcher) Dispatch(events ...port.DomainEvent) {
	s.events = append(s.events, events...)
}

type testTxScopeForOutboxEvents struct {
	outbox port.OutboxAppender
}

func (s testTxScopeForOutboxEvents) Context() context.Context { return context.Background() }
func (s testTxScopeForOutboxEvents) AfterCommit(fn func() error) {
	if fn != nil {
		_ = fn()
	}
}
func (s testTxScopeForOutboxEvents) UserRepository() port.UserRepository                 { return nil }
func (s testTxScopeForOutboxEvents) BoardRepository() port.BoardRepository               { return nil }
func (s testTxScopeForOutboxEvents) PostRepository() port.PostRepository                 { return nil }
func (s testTxScopeForOutboxEvents) TagRepository() port.TagRepository                   { return nil }
func (s testTxScopeForOutboxEvents) PostTagRepository() port.PostTagRepository           { return nil }
func (s testTxScopeForOutboxEvents) CommentRepository() port.CommentRepository           { return nil }
func (s testTxScopeForOutboxEvents) ReactionRepository() port.ReactionRepository         { return nil }
func (s testTxScopeForOutboxEvents) AttachmentRepository() port.AttachmentRepository     { return nil }
func (s testTxScopeForOutboxEvents) ReportRepository() port.ReportRepository             { return nil }
func (s testTxScopeForOutboxEvents) NotificationRepository() port.NotificationRepository { return nil }
func (s testTxScopeForOutboxEvents) EmailVerificationTokenRepository() port.EmailVerificationTokenRepository {
	return nil
}
func (s testTxScopeForOutboxEvents) PasswordResetTokenRepository() port.PasswordResetTokenRepository {
	return nil
}
func (s testTxScopeForOutboxEvents) Outbox() port.OutboxAppender { return s.outbox }

func TestDispatchDomainActions_UsesOutboxWithinTransaction(t *testing.T) {
	outbox := &spyOutboxAppender{}
	dispatcher := &spyActionDispatcher{}
	tx := testTxScopeForOutboxEvents{outbox: outbox}

	err := svccommon.DispatchDomainActions(tx, dispatcher, appevent.NewBoardChanged("created", 10))
	require.NoError(t, err)
	require.Len(t, outbox.messages, 1)
	assert.Empty(t, dispatcher.events)
	assert.Equal(t, port.OutboxStatusPending, outbox.messages[0].Status)
	assert.WithinDuration(t, time.Now(), outbox.messages[0].OccurredAt, time.Second)
}

func TestDispatchDomainActions_FallsBackToDispatcherWithoutOutbox(t *testing.T) {
	dispatcher := &spyActionDispatcher{}
	tx := testTxScopeForOutboxEvents{outbox: nil}

	err := svccommon.DispatchDomainActions(tx, dispatcher, appevent.NewBoardChanged("created", 10))
	require.NoError(t, err)
	require.Len(t, dispatcher.events, 1)
}
