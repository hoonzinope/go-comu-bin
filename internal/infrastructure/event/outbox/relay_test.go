package outbox

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	inmemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
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

func (s *fakeOutboxStore) SelectByID(id string) (*port.OutboxMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, message := range s.ready {
		if message.ID != id {
			continue
		}
		copied := message
		return &copied, nil
	}
	return nil, nil
}

func (s *fakeOutboxStore) SelectDead(limit int, _ string) ([]port.OutboxMessage, error) {
	_ = limit
	return nil, nil
}

func (s *fakeOutboxStore) RenewProcessing(id string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for idx := range s.ready {
		if s.ready[idx].ID == id {
			s.ready[idx].Status = port.OutboxStatusProcessing
		}
	}
	return nil
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
	fn func(ctx context.Context, event port.DomainEvent) error
}

func (h testHandler) Handle(ctx context.Context, event port.DomainEvent) error {
	return h.fn(ctx, event)
}

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
	relay := NewRelay(store, serializer, slog.New(slog.NewTextHandler(io.Discard, nil)), RelayConfig{
		WorkerCount:  1,
		BatchSize:    10,
		PollInterval: time.Millisecond,
		MaxAttempts:  3,
		BaseBackoff:  time.Millisecond,
	})
	handled := make(chan struct{}, 1)
	relay.Subscribe(appevent.EventNameBoardChanged, testHandler{fn: func(ctx context.Context, event port.DomainEvent) error {
		_ = ctx
		handled <- struct{}{}
		return nil
	}})

	processed := relay.pollOnce(context.Background(), time.Now())
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
	relay := NewRelay(store, serializer, slog.New(slog.NewTextHandler(io.Discard, nil)), RelayConfig{
		WorkerCount:  1,
		BatchSize:    10,
		PollInterval: time.Millisecond,
		MaxAttempts:  3,
		BaseBackoff:  time.Millisecond,
	})
	relay.Subscribe(appevent.EventNameBoardChanged, testHandler{fn: func(ctx context.Context, event port.DomainEvent) error {
		_ = ctx
		return errors.New("handler failed")
	}})

	processed := relay.pollOnce(context.Background(), time.Now())
	assert.True(t, processed)
	assert.Equal(t, []string{"m1"}, store.retryCalls)
	assert.Equal(t, []string{"m2"}, store.deadCalls)
	assert.Empty(t, store.succeededIDs)
}

func TestRelay_Start_PropagatesContextToHandler(t *testing.T) {
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
	relay := NewRelay(store, serializer, slog.New(slog.NewTextHandler(io.Discard, nil)), RelayConfig{
		WorkerCount:  1,
		BatchSize:    10,
		PollInterval: 10 * time.Millisecond,
		MaxAttempts:  3,
		BaseBackoff:  time.Millisecond,
	})

	handled := make(chan string, 1)
	relay.Subscribe(appevent.EventNameBoardChanged, testHandler{fn: func(ctx context.Context, event port.DomainEvent) error {
		_ = event
		v, _ := ctx.Value("request_id").(string)
		handled <- v
		return nil
	}})

	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), "request_id", "relay-1"))
	defer cancel()
	relay.Start(ctx)

	select {
	case got := <-handled:
		assert.Equal(t, "relay-1", got)
	case <-time.After(time.Second):
		t.Fatal("handler was not called")
	}
	cancel()
	relay.Wait()
}

func TestRelay_Start_DoesNotRedispatchWhileHandlerStillRunning(t *testing.T) {
	serializer := appevent.NewJSONEventSerializer()
	store := inmemory.NewOutboxRepository(inmemory.WithProcessingTimeout(20 * time.Millisecond))
	name, payload, at, err := serializer.Serialize(appevent.NewBoardChanged("created", 1))
	require.NoError(t, err)
	require.NoError(t, store.Append(port.OutboxMessage{
		ID:            "m1",
		EventName:     name,
		Payload:       payload,
		OccurredAt:    at,
		NextAttemptAt: at,
		Status:        port.OutboxStatusPending,
	}))

	relay := NewRelay(store, serializer, slog.New(slog.NewTextHandler(io.Discard, nil)), RelayConfig{
		WorkerCount:     2,
		BatchSize:       1,
		PollInterval:    time.Millisecond,
		MaxAttempts:     3,
		BaseBackoff:     time.Millisecond,
		ProcessingLease: 20 * time.Millisecond,
		LeaseRefresh:    5 * time.Millisecond,
	})

	var mu sync.Mutex
	callCount := 0
	firstCallStarted := make(chan struct{}, 1)
	relay.Subscribe(appevent.EventNameBoardChanged, testHandler{fn: func(ctx context.Context, event port.DomainEvent) error {
		_ = ctx
		_ = event
		mu.Lock()
		callCount++
		current := callCount
		mu.Unlock()
		if current == 1 {
			firstCallStarted <- struct{}{}
		}
		time.Sleep(70 * time.Millisecond)
		return nil
	}})

	ctx, cancel := context.WithCancel(context.Background())
	relay.Start(ctx)
	select {
	case <-firstCallStarted:
	case <-time.After(time.Second):
		t.Fatal("handler was not called")
	}
	time.Sleep(120 * time.Millisecond)
	cancel()
	relay.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, callCount)
}

func TestRelay_PollOnce_RecoversFromHandlerPanic(t *testing.T) {
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
	var logBuf bytes.Buffer
	relay := NewRelay(store, serializer, slog.New(slog.NewJSONHandler(&logBuf, nil)), RelayConfig{
		WorkerCount:  1,
		BatchSize:    10,
		PollInterval: time.Millisecond,
		MaxAttempts:  3,
		BaseBackoff:  time.Millisecond,
	})
	relay.Subscribe(appevent.EventNameBoardChanged, testHandler{fn: func(ctx context.Context, event port.DomainEvent) error {
		_ = ctx
		_ = event
		panic("boom")
	}})

	processed := relay.pollOnce(context.Background(), time.Now())
	assert.True(t, processed)
	assert.Equal(t, []string{"m1"}, store.retryCalls)
	assert.Contains(t, logBuf.String(), "dispatch outbox event failed")
	assert.Contains(t, logBuf.String(), "\"panic\":\"boom\"")
	assert.Contains(t, logBuf.String(), "\"stack\"")
}
