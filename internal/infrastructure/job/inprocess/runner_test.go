package inprocess

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTicker struct {
	ch      chan time.Time
	stopped bool
	mu      sync.Mutex
}

func newStubTicker() *stubTicker {
	return &stubTicker{ch: make(chan time.Time, 4)}
}

func (t *stubTicker) C() <-chan time.Time { return t.ch }

func (t *stubTicker) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopped = true
}

type stubTickerFactory struct {
	mu      sync.Mutex
	tickers []*stubTicker
}

func (f *stubTickerFactory) New(time.Duration) ticker {
	ticker := newStubTicker()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tickers = append(f.tickers, ticker)
	return ticker
}

func (f *stubTickerFactory) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.tickers)
}

func (f *stubTickerFactory) At(i int) *stubTicker {
	f.mu.Lock()
	defer f.mu.Unlock()
	if i < 0 || i >= len(f.tickers) {
		return nil
	}
	return f.tickers[i]
}

func TestRunner_Start_RunsRegisteredJobOnTick(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	factory := &stubTickerFactory{}
	runner := NewRunner(logger, WithTickerFactory(factory.New))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runCh := make(chan struct{}, 1)
	require.NoError(t, runner.Register(Job{
		Name:     "orphan-cleanup",
		Interval: time.Second,
		Run: func(context.Context) error {
			runCh <- struct{}{}
			return nil
		},
	}))

	runner.Start(ctx)
	require.Eventually(t, func() bool {
		return factory.Len() == 1
	}, time.Second, 10*time.Millisecond)
	ticker := factory.At(0)
	require.NotNil(t, ticker)
	ticker.ch <- time.Now()

	select {
	case <-runCh:
	case <-time.After(time.Second):
		t.Fatal("job did not run")
	}
}

func TestRunner_Start_StopsTickerOnContextDone(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	factory := &stubTickerFactory{}
	runner := NewRunner(logger, WithTickerFactory(factory.New))
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, runner.Register(Job{
		Name:     "orphan-cleanup",
		Interval: time.Second,
		Run:      func(context.Context) error { return nil },
	}))

	runner.Start(ctx)
	require.Eventually(t, func() bool {
		return factory.Len() == 1
	}, time.Second, 10*time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)

	ticker := factory.At(0)
	require.NotNil(t, ticker)
	ticker.mu.Lock()
	stopped := ticker.stopped
	ticker.mu.Unlock()
	assert.True(t, stopped)
}

func TestRunner_Register_InvalidJobReturnsError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runner := NewRunner(logger)

	err := runner.Register(Job{Name: "", Interval: time.Second, Run: func(context.Context) error { return errors.New("x") }})
	require.Error(t, err)
}

func TestRunner_Start_RecoversFromJobPanic(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	factory := &stubTickerFactory{}
	runner := NewRunner(logger, WithTickerFactory(factory.New))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runCh := make(chan struct{}, 1)
	callCount := 0
	require.NoError(t, runner.Register(Job{
		Name:     "panic-job",
		Interval: time.Second,
		Run: func(context.Context) error {
			callCount++
			if callCount == 1 {
				panic("boom")
			}
			runCh <- struct{}{}
			return nil
		},
	}))

	runner.Start(ctx)
	require.Eventually(t, func() bool {
		return factory.Len() == 1
	}, time.Second, 10*time.Millisecond)
	ticker := factory.At(0)
	require.NotNil(t, ticker)
	ticker.ch <- time.Now()
	ticker.ch <- time.Now()

	select {
	case <-runCh:
	case <-time.After(time.Second):
		t.Fatal("job did not recover after panic")
	}
	assert.Contains(t, logBuf.String(), "background job panicked")
	assert.Contains(t, logBuf.String(), "\"panic\":\"boom\"")
}
