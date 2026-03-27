package outbox

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

type RelayConfig struct {
	WorkerCount      int
	BatchSize        int
	PollInterval     time.Duration
	MaxAttempts      int
	BaseBackoff      time.Duration
	MaxBackoffFactor int
	ProcessingLease  time.Duration
	LeaseRefresh     time.Duration
}

type Relay struct {
	store      port.OutboxStore
	serializer port.EventSerializer
	logger     *slog.Logger
	cfg        RelayConfig

	mu       sync.RWMutex
	handlers map[string][]port.EventHandler
	wg       sync.WaitGroup
}

func NewRelay(store port.OutboxStore, serializer port.EventSerializer, logger *slog.Logger, cfg RelayConfig) *Relay {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 1
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 100 * time.Millisecond
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = 100 * time.Millisecond
	}
	if cfg.MaxBackoffFactor <= 0 {
		cfg.MaxBackoffFactor = 64
	}
	if cfg.ProcessingLease <= 0 {
		cfg.ProcessingLease = 30 * time.Second
	}
	if cfg.LeaseRefresh <= 0 {
		cfg.LeaseRefresh = cfg.ProcessingLease / 3
	}
	if cfg.LeaseRefresh <= 0 {
		cfg.LeaseRefresh = time.Second
	}
	return &Relay{
		store:      store,
		serializer: serializer,
		logger:     logger,
		cfg:        cfg,
		handlers:   make(map[string][]port.EventHandler),
	}
}

func (r *Relay) Subscribe(eventName string, handler port.EventHandler) {
	if handler == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[eventName] = append(r.handlers[eventName], handler)
}

func (r *Relay) Start(ctx context.Context) {
	if r == nil || r.store == nil || r.serializer == nil {
		return
	}
	for i := 0; i < r.cfg.WorkerCount; i++ {
		r.wg.Add(1)
		go r.worker(ctx)
	}
}

func (r *Relay) Wait() {
	if r == nil {
		return
	}
	r.wg.Wait()
}

func (r *Relay) worker(ctx context.Context) {
	defer r.wg.Done()

	ticker := time.NewTicker(r.cfg.PollInterval)
	defer ticker.Stop()

	for {
		if ctx.Err() != nil {
			return
		}
		shouldWait := true
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					r.warn(
						"outbox relay panicked",
						"panic", recovered,
						"stack", string(debug.Stack()),
					)
				}
			}()
			if processed := r.pollOnce(ctx, time.Now()); processed {
				shouldWait = false
			}
		}()
		if ctx.Err() != nil {
			return
		}
		if !shouldWait {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Relay) pollOnce(ctx context.Context, now time.Time) bool {
	messages, err := r.store.FetchReady(r.cfg.BatchSize, now)
	if err != nil {
		r.warn("fetch outbox ready messages failed", "error", err)
		return false
	}
	if len(messages) == 0 {
		return false
	}
	for _, message := range messages {
		r.handleMessage(ctx, message, now)
	}
	return true
}

func (r *Relay) handleMessage(ctx context.Context, message port.OutboxMessage, now time.Time) {
	event, err := r.serializer.Deserialize(message.EventName, message.Payload, message.OccurredAt)
	if err != nil {
		r.markFailure(message, now, "deserialize outbox event failed", err)
		return
	}
	stopRenew := r.startLeaseRenewal(ctx, message.ID)
	defer stopRenew()
	if err := r.dispatch(ctx, event); err != nil {
		r.markFailure(message, now, "dispatch outbox event failed", err)
		return
	}
	if err := r.store.MarkSucceeded(message.ID); err != nil {
		r.warn("mark outbox message succeeded failed", "id", message.ID, "error", err)
	}
}

func (r *Relay) startLeaseRenewal(ctx context.Context, messageID string) func() {
	if r.cfg.LeaseRefresh <= 0 || r.cfg.ProcessingLease <= 0 || messageID == "" {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(r.cfg.LeaseRefresh)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				nextAttemptAt := time.Now().Add(r.cfg.ProcessingLease)
				if err := r.store.RenewProcessing(messageID, nextAttemptAt); err != nil {
					r.warn("renew outbox message lease failed", "id", messageID, "error", err)
				}
			}
		}
	}()
	return func() {
		close(done)
	}
}

func (r *Relay) dispatch(ctx context.Context, event port.DomainEvent) error {
	if event == nil {
		return nil
	}
	handlers := r.handlersFor(event.EventName())
	for _, handler := range handlers {
		if handler == nil {
			continue
		}
		if err := callHandler(ctx, handler, event); err != nil {
			return err
		}
	}
	return nil
}

func (r *Relay) handlersFor(eventName string) []port.EventHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handlers := r.handlers[eventName]
	if len(handlers) == 0 {
		return nil
	}
	out := make([]port.EventHandler, len(handlers))
	copy(out, handlers)
	return out
}

func (r *Relay) markFailure(message port.OutboxMessage, now time.Time, msg string, err error) {
	attempt := message.AttemptCount
	args := []any{"id", message.ID, "event", message.EventName, "attempt", attempt, "error", err}
	if panicErr, ok := err.(panicError); ok {
		args = append(args, "panic", panicErr.value, "stack", panicErr.Stack())
	}
	if attempt >= r.cfg.MaxAttempts {
		if markErr := r.store.MarkDead(message.ID, err.Error()); markErr != nil {
			r.warn("mark outbox message dead failed", "id", message.ID, "error", markErr)
		}
		r.warn(msg, append(args, "status", "dead")...)
		return
	}
	nextAttemptAt := now.Add(backoffDuration(r.cfg.BaseBackoff, attempt, r.cfg.MaxBackoffFactor))
	if markErr := r.store.MarkRetry(message.ID, nextAttemptAt, err.Error()); markErr != nil {
		r.warn("mark outbox message retry failed", "id", message.ID, "error", markErr)
		return
	}
	r.warn(msg, append(args, "status", "retry")...)
}

func backoffDuration(base time.Duration, attempt int, maxFactor int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	factor := 1 << (attempt - 1)
	if factor > maxFactor {
		factor = maxFactor
	}
	return time.Duration(factor) * base
}

func callHandler(ctx context.Context, handler port.EventHandler, event port.DomainEvent) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = panicError{value: recovered, stack: debug.Stack()}
		}
	}()
	return handler.Handle(ctx, event)
}

type panicError struct {
	value any
	stack []byte
}

func (e panicError) Error() string {
	return "event handler panic"
}

func (e panicError) Stack() string {
	return string(e.stack)
}

func (r *Relay) warn(msg string, args ...any) {
	if r == nil || r.logger == nil {
		return
	}
	r.logger.Warn(msg, args...)
}
