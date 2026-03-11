package inprocess

import (
	"sync"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.EventBus = (*EventBus)(nil)
var _ port.EventPublisher = (*EventBus)(nil)

type EventBus struct {
	mu       sync.RWMutex
	handlers map[string][]port.EventHandler
	logger   port.Logger
}

func NewEventBus(logger port.Logger) *EventBus {
	return &EventBus{
		handlers: make(map[string][]port.EventHandler),
		logger:   logger,
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
	go b.dispatch(events)
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
			b.logger.Warn("event handler panic", "event", event.EventName(), "panic", recovered)
		}
	}()
	if err := handler.Handle(event); err != nil {
		b.logger.Warn("event handler failed", "event", event.EventName(), "error", err)
	}
}
