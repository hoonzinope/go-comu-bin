package inprocess

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEvent struct {
	name string
	at   time.Time
}

func (e testEvent) EventName() string {
	return e.name
}

func (e testEvent) OccurredAt() time.Time {
	return e.at
}

type testHandler struct {
	fn func(event port.DomainEvent) error
}

func (h testHandler) Handle(event port.DomainEvent) error {
	return h.fn(event)
}

type spyLogger struct {
	mu        sync.Mutex
	warnCount int
}

func (l *spyLogger) Warn(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warnCount++
}

func (l *spyLogger) Error(msg string, args ...any) {}

func (l *spyLogger) WarnCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.warnCount
}

func TestEventBus_PublishIsAsync(t *testing.T) {
	logger := &spyLogger{}
	bus := NewEventBus(logger)
	start := make(chan struct{})
	release := make(chan struct{})

	bus.Subscribe("test", testHandler{fn: func(event port.DomainEvent) error {
		close(start)
		<-release
		return nil
	}})

	bus.Publish(testEvent{name: "test", at: time.Now()})

	select {
	case <-start:
	case <-time.After(time.Second):
		t.Fatal("handler was not started")
	}

	close(release)
}

func TestEventBus_RecoversFromPanicAndContinues(t *testing.T) {
	logger := &spyLogger{}
	bus := NewEventBus(logger)
	called := make(chan struct{}, 1)

	bus.Subscribe("test", testHandler{fn: func(event port.DomainEvent) error {
		panic("boom")
	}})
	bus.Subscribe("test", testHandler{fn: func(event port.DomainEvent) error {
		called <- struct{}{}
		return nil
	}})

	bus.Publish(testEvent{name: "test", at: time.Now()})

	require.Eventually(t, func() bool {
		return len(called) == 1
	}, time.Second, 10*time.Millisecond)
	assert.GreaterOrEqual(t, logger.WarnCount(), 1)
}

func TestEventBus_LogsHandlerError(t *testing.T) {
	logger := &spyLogger{}
	bus := NewEventBus(logger)

	bus.Subscribe("test", testHandler{fn: func(event port.DomainEvent) error {
		return errors.New("handler failed")
	}})

	bus.Publish(testEvent{name: "test", at: time.Now()})

	require.Eventually(t, func() bool {
		return logger.WarnCount() >= 1
	}, time.Second, 10*time.Millisecond)
}
