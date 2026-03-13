package outbox

import (
	"errors"
	"sync"
	"testing"
	"time"

	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOutboxStore struct {
	mu            sync.Mutex
	ready         []port.OutboxMessage
	succeededIDs  []string
	retryCalls    []string
	deadCalls     []string
	markRetryErrs []string
}

func (s *fakeOutboxStore) Append(messages ...port.OutboxMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = append(s.ready, messages...)
	return nil
}

func (s *fakeOutboxStore) FetchReady(limit int, _ time.Time) ([]port.OutboxMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || len(s.ready) == 0 {
		return nil, nil
	}
	if limit > len(s.ready) {
		limit = len(s.ready)
	}
	out := make([]port.OutboxMessage, limit)
	copy(out, s.ready[:limit])
	s.ready = s.ready[limit:]
	return out, nil
}

func (s *fakeOutboxStore) MarkSucceeded(ids ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.succeededIDs = append(s.succeededIDs, ids...)
	return nil
}

func (s *fakeOutboxStore) MarkRetry(id string, _ time.Time, err string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retryCalls = append(s.retryCalls, id)
	s.markRetryErrs = append(s.markRetryErrs, err)
	return nil
}

func (s *fakeOutboxStore) MarkDead(id string, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deadCalls = append(s.deadCalls, id)
	return nil
}

type testHandler struct {
	fn func(event port.DomainEvent) error
}

func (h testHandler) Handle(event port.DomainEvent) error {
	return h.fn(event)
}

type noopLogger struct{}

func (noopLogger) Warn(msg string, args ...any)  {}
func (noopLogger) Error(msg string, args ...any) {}

func TestRelay_PollOnce_MarkSucceededOnSuccess(t *testing.T) {
	serializer := appevent.NewJSONEventSerializer()
	name, payload, at, err := serializer.Serialize(appevent.NewBoardChanged("created", 1))
	require.NoError(t, err)

	store := &fakeOutboxStore{
		ready: []port.OutboxMessage{{
			ID:           "m1",
			EventName:    name,
			Payload:      payload,
			OccurredAt:   at,
			AttemptCount: 1,
		}},
	}
	relay := NewRelay(store, serializer, noopLogger{}, RelayConfig{
		WorkerCount:  1,
		BatchSize:    10,
		PollInterval: time.Millisecond,
		MaxAttempts:  3,
		BaseBackoff:  time.Millisecond,
	})
	handled := make(chan struct{}, 1)
	relay.Subscribe(appevent.EventNameBoardChanged, testHandler{fn: func(event port.DomainEvent) error {
		handled <- struct{}{}
		return nil
	}})

	processed := relay.pollOnce(time.Now())
	assert.True(t, processed)
	select {
	case <-handled:
	default:
		t.Fatal("handler was not called")
	}
	assert.Equal(t, []string{"m1"}, store.succeededIDs)
	assert.Empty(t, store.retryCalls)
	assert.Empty(t, store.deadCalls)
}

func TestRelay_PollOnce_RetryThenDead(t *testing.T) {
	serializer := appevent.NewJSONEventSerializer()
	name, payload, at, err := serializer.Serialize(appevent.NewBoardChanged("created", 1))
	require.NoError(t, err)

	store := &fakeOutboxStore{
		ready: []port.OutboxMessage{
			{ID: "m1", EventName: name, Payload: payload, OccurredAt: at, AttemptCount: 1},
			{ID: "m2", EventName: name, Payload: payload, OccurredAt: at, AttemptCount: 3},
		},
	}
	relay := NewRelay(store, serializer, noopLogger{}, RelayConfig{
		WorkerCount:  1,
		BatchSize:    10,
		PollInterval: time.Millisecond,
		MaxAttempts:  3,
		BaseBackoff:  time.Millisecond,
	})
	relay.Subscribe(appevent.EventNameBoardChanged, testHandler{fn: func(event port.DomainEvent) error {
		return errors.New("handler failed")
	}})

	processed := relay.pollOnce(time.Now())
	assert.True(t, processed)
	assert.Equal(t, []string{"m1"}, store.retryCalls)
	assert.Equal(t, []string{"m2"}, store.deadCalls)
	assert.Empty(t, store.succeededIDs)
}
