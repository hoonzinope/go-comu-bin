package inprocess

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.EventBus = (*EventBus)(nil)
var _ port.EventPublisher = (*EventBus)(nil)

type EventBus struct {
	mu             sync.RWMutex
	handlers       map[string][]port.EventHandler
	logger         port.Logger
	queue          chan []port.DomainEvent
	workerCount    int
	enqueueTimeout time.Duration
	after          func(time.Duration) <-chan time.Time
	lifecycleMu    sync.RWMutex
	closed         bool
	wg             sync.WaitGroup
	enqueued       atomic.Uint64
	dropped        atomic.Uint64
}

type Option func(*EventBus)

type Stats struct {
	EnqueuedCount uint64
	DroppedCount  uint64
}

const (
	defaultQueueSize      = 256
	defaultWorkerCount    = 1
	defaultEnqueueTimeout = 100 * time.Millisecond
)

func NewEventBus(logger port.Logger, opts ...Option) *EventBus {
	bus := &EventBus{
		handlers:       make(map[string][]port.EventHandler),
		logger:         logger,
		queue:          make(chan []port.DomainEvent, defaultQueueSize),
		workerCount:    defaultWorkerCount,
		enqueueTimeout: defaultEnqueueTimeout,
		after:          time.After,
	}
	for _, opt := range opts {
		opt(bus)
	}
	for i := 0; i < bus.workerCount; i++ {
		bus.wg.Add(1)
		go bus.worker()
	}
	return bus
}

func WithQueueSize(size int) Option {
	return func(b *EventBus) {
		if size > 0 {
			b.queue = make(chan []port.DomainEvent, size)
		}
	}
}

func WithWorkerCount(count int) Option {
	return func(b *EventBus) {
		if count > 0 {
			b.workerCount = count
		}
	}
}

func WithEnqueueTimeout(timeout time.Duration) Option {
	return func(b *EventBus) {
		if timeout > 0 {
			b.enqueueTimeout = timeout
		}
	}
}

func (b *EventBus) Subscribe(eventName string, handler port.EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventName] = append(b.handlers[eventName], handler)
}

func (b *EventBus) Publish(events ...port.DomainEvent) {
	if len(events) == 0 {
		return
	}
	copied := append([]port.DomainEvent(nil), events...)
	b.lifecycleMu.RLock()
	defer b.lifecycleMu.RUnlock()
	if b.closed {
		b.dropped.Add(1)
		b.warn("event bus closed; dropping events", "count", len(copied))
		return
	}
	select {
	case b.queue <- copied:
		b.enqueued.Add(1)
		return
	default:
	}
	timeout := b.enqueueTimeout
	select {
	case b.queue <- copied:
		b.enqueued.Add(1)
	case <-b.after(timeout):
		b.dropped.Add(1)
		b.warn("event queue enqueue timeout; dropping events", "count", len(copied), "timeout", timeout)
	}
}

func (b *EventBus) worker() {
	defer b.wg.Done()
	for events := range b.queue {
		b.dispatch(events)
	}
}

func (b *EventBus) dispatch(events []port.DomainEvent) {
	for _, event := range events {
		if event == nil {
			continue
		}
		handlers := b.handlersFor(event.EventName())
		for _, handler := range handlers {
			b.callHandler(handler, event)
		}
	}
}

func (b *EventBus) handlersFor(eventName string) []port.EventHandler {
	b.mu.RLock()
	defer b.mu.RUnlock()
	handlers := b.handlers[eventName]
	if len(handlers) == 0 {
		return nil
	}
	out := make([]port.EventHandler, len(handlers))
	copy(out, handlers)
	return out
}

func (b *EventBus) callHandler(handler port.EventHandler, event port.DomainEvent) {
	defer func() {
		if recovered := recover(); recovered != nil {
			b.warn("event handler panic", "event", event.EventName(), "panic", recovered)
		}
	}()
	if err := handler.Handle(event); err != nil {
		b.warn("event handler failed", "event", event.EventName(), "error", err)
	}
}

func (b *EventBus) warn(msg string, args ...any) {
	if b == nil || b.logger == nil {
		return
	}
	b.logger.Warn(msg, args...)
}

func (b *EventBus) Stats() Stats {
	if b == nil {
		return Stats{}
	}
	return Stats{
		EnqueuedCount: b.enqueued.Load(),
		DroppedCount:  b.dropped.Load(),
	}
}

func (b *EventBus) Close() {
	if b == nil {
		return
	}
	b.lifecycleMu.Lock()
	if b.closed {
		b.lifecycleMu.Unlock()
		return
	}
	b.closed = true
	close(b.queue)
	b.lifecycleMu.Unlock()
	b.wg.Wait()
}
