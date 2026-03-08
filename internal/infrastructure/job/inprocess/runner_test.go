package inprocess

import (
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
	tickers []*stubTicker
}

func (f *stubTickerFactory) New(time.Duration) ticker {
	ticker := newStubTicker()
	f.tickers = append(f.tickers, ticker)
	return ticker
}

func TestRunner_Start_RunsRegisteredJobOnTick(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	factory := &stubTickerFactory{}
	runner := NewRunner(logger, WithTickerFactory(factory.New))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runCh := make(chan struct{}, 1)
	runner.Register(Job{
		Name:     "orphan-cleanup",
		Interval: time.Second,
		Run: func(context.Context) error {
			runCh <- struct{}{}
			return nil
		},
	})

	runner.Start(ctx)
	require.Eventually(t, func() bool {
		return len(factory.tickers) == 1
	}, time.Second, 10*time.Millisecond)
	factory.tickers[0].ch <- time.Now()

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
	runner.Register(Job{
		Name:     "orphan-cleanup",
		Interval: time.Second,
		Run:      func(context.Context) error { return nil },
	})

	runner.Start(ctx)
	require.Eventually(t, func() bool {
		return len(factory.tickers) == 1
	}, time.Second, 10*time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)

	factory.tickers[0].mu.Lock()
	stopped := factory.tickers[0].stopped
	factory.tickers[0].mu.Unlock()
	assert.True(t, stopped)
}

func TestRunner_Register_InvalidJobPanics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runner := NewRunner(logger)

	assert.Panics(t, func() {
		runner.Register(Job{Name: "", Interval: time.Second, Run: func(context.Context) error { return errors.New("x") }})
	})
}
