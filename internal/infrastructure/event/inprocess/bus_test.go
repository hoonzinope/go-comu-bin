package inprocess

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
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
	warnCount atomic.Int32
}

func (l *spyLogger) WarnCount() int {
	return int(l.warnCount.Load())
}

func (l *spyLogger) Logger() *slog.Logger {
	return slog.New(&spyHandler{logger: l})
}

type spyHandler struct {
	logger *spyLogger
}

func (h *spyHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *spyHandler) Handle(_ context.Context, record slog.Record) error {
	if record.Level >= slog.LevelWarn {
		h.logger.warnCount.Add(1)
	}
	return nil
}

func (h *spyHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *spyHandler) WithGroup(string) slog.Handler { return h }

func TestEventBus_PublishIsAsync(t *testing.T) {
	logger := &spyLogger{}
	bus := NewEventBus(logger.Logger())
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
	bus := NewEventBus(logger.Logger())
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
	bus := NewEventBus(logger.Logger())

	bus.Subscribe("test", testHandler{fn: func(event port.DomainEvent) error {
		return errors.New("handler failed")
	}})

	bus.Publish(testEvent{name: "test", at: time.Now()})

	require.Eventually(t, func() bool {
		return logger.WarnCount() >= 1
	}, time.Second, 10*time.Millisecond)
}

func TestEventBus_DropsWhenQueueIsFull(t *testing.T) {
	logger := &spyLogger{}
	bus := NewEventBus(logger.Logger(), WithQueueSize(1), WithWorkerCount(1), WithEnqueueTimeout(20*time.Millisecond))

	started := make(chan struct{})
	release := make(chan struct{})
	bus.Subscribe("test", testHandler{fn: func(event port.DomainEvent) error {
		close(started)
		<-release
		return nil
	}})

	bus.Publish(testEvent{name: "test", at: time.Now()})
	<-started

	// First buffered publish should enter queue, second one should overflow and be dropped.
	bus.Publish(testEvent{name: "test", at: time.Now()})
	bus.Publish(testEvent{name: "test", at: time.Now()})

	require.Eventually(t, func() bool {
		return logger.WarnCount() >= 1
	}, time.Second, 10*time.Millisecond)
	assert.GreaterOrEqual(t, bus.Stats().DroppedCount, uint64(1))
	close(release)
}

func TestEventBus_PublishBlocksUntilQueueHasSpace(t *testing.T) {
	logger := &spyLogger{}
	bus := NewEventBus(logger.Logger(), WithQueueSize(1), WithWorkerCount(1), WithEnqueueTimeout(time.Second))

	started := make(chan struct{})
	release := make(chan struct{})
	bus.Subscribe("test", testHandler{fn: func(event port.DomainEvent) error {
		close(started)
		<-release
		return nil
	}})

	bus.Publish(testEvent{name: "test", at: time.Now()})
	<-started
	bus.Publish(testEvent{name: "test", at: time.Now()})

	done := make(chan struct{})
	go func() {
		bus.Publish(testEvent{name: "test", at: time.Now()})
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("publish should block while queue is full")
	case <-time.After(30 * time.Millisecond):
	}

	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publish did not proceed after queue drained")
	}
}

func TestEventBus_Close_WaitsForInFlightHandlers(t *testing.T) {
	logger := &spyLogger{}
	bus := NewEventBus(logger.Logger(), WithQueueSize(4), WithWorkerCount(1))

	started := make(chan struct{})
	release := make(chan struct{})
	bus.Subscribe("test", testHandler{fn: func(event port.DomainEvent) error {
		close(started)
		<-release
		return nil
	}})

	bus.Publish(testEvent{name: "test", at: time.Now()})
	<-started

	closed := make(chan struct{})
	go func() {
		bus.Close()
		close(closed)
	}()

	select {
	case <-closed:
		t.Fatal("close returned before in-flight handler finished")
	case <-time.After(30 * time.Millisecond):
	}

	close(release)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("close did not return after handler completed")
	}
}

func TestEventBus_PublishAfterClose_IsDropped(t *testing.T) {
	logger := &spyLogger{}
	bus := NewEventBus(logger.Logger(), WithQueueSize(1), WithWorkerCount(1))
	bus.Close()

	bus.Publish(testEvent{name: "test", at: time.Now()})

	stats := bus.Stats()
	assert.Equal(t, uint64(1), stats.DroppedCount)
	assert.GreaterOrEqual(t, logger.WarnCount(), 1)
}

func TestEventBus_Publish_DoesNotCreateTimeoutTimerWhenQueueHasCapacity(t *testing.T) {
	logger := &spyLogger{}
	bus := NewEventBus(logger.Logger(), WithQueueSize(4), WithWorkerCount(1), WithEnqueueTimeout(time.Second))
	defer bus.Close()

	var afterCalls atomic.Int64
	bus.after = func(d time.Duration) <-chan time.Time {
		afterCalls.Add(1)
		return time.After(d)
	}

	bus.Publish(testEvent{name: "test", at: time.Now()})

	require.Eventually(t, func() bool {
		return bus.Stats().EnqueuedCount >= 1
	}, time.Second, 10*time.Millisecond)
	assert.Equal(t, int64(0), afterCalls.Load())
}

func TestEventBus_CloseDoesNotWaitForEnqueueTimeout(t *testing.T) {
	logger := &spyLogger{}
	timeout := 1500 * time.Millisecond
	bus := NewEventBus(logger.Logger(), WithQueueSize(1), WithWorkerCount(1), WithEnqueueTimeout(timeout))

	started := make(chan struct{})
	release := make(chan struct{})
	bus.Subscribe("test", testHandler{fn: func(event port.DomainEvent) error {
		close(started)
		<-release
		return nil
	}})

	bus.Publish(testEvent{name: "test", at: time.Now()})
	<-started
	bus.Publish(testEvent{name: "test", at: time.Now()})

	publishDone := make(chan struct{})
	go func() {
		bus.Publish(testEvent{name: "test", at: time.Now()})
		close(publishDone)
	}()

	select {
	case <-publishDone:
		t.Fatal("publish should block while queue is full")
	case <-time.After(30 * time.Millisecond):
	}

	closeDone := make(chan time.Duration, 1)
	go func() {
		begin := time.Now()
		bus.Close()
		closeDone <- time.Since(begin)
	}()

	select {
	case <-publishDone:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("blocked publish did not unblock promptly after close signal")
	}

	close(release)

	select {
	case elapsed := <-closeDone:
		if elapsed >= timeout {
			t.Fatalf("close waited for enqueue timeout: elapsed=%s timeout=%s", elapsed, timeout)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not return promptly")
	}
}
