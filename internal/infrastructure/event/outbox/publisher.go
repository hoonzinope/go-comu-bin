package outbox

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.EventPublisher = (*Publisher)(nil)

type Publisher struct {
	store      port.OutboxAppender
	serializer port.EventSerializer
	logger     *slog.Logger
}

func NewPublisher(store port.OutboxAppender, serializer port.EventSerializer, logger *slog.Logger) *Publisher {
	return &Publisher{
		store:      store,
		serializer: serializer,
		logger:     logger,
	}
}

func (p *Publisher) Publish(events ...port.DomainEvent) {
	if p == nil || p.store == nil || p.serializer == nil || len(events) == 0 {
		return
	}
	messages := make([]port.OutboxMessage, 0, len(events))
	for _, event := range events {
		eventName, payload, occurredAt, err := p.serializer.Serialize(event)
		if err != nil {
			p.warn("serialize event for outbox failed", "error", err)
			continue
		}
		id, err := newOutboxMessageID()
		if err != nil {
			p.warn("generate outbox message id failed", "error", err)
			continue
		}
		if occurredAt.IsZero() {
			occurredAt = time.Now()
		}
		messages = append(messages, port.OutboxMessage{
			ID:            id,
			EventName:     eventName,
			Payload:       payload,
			OccurredAt:    occurredAt,
			NextAttemptAt: occurredAt,
			Status:        port.OutboxStatusPending,
		})
	}
	if len(messages) == 0 {
		return
	}
	if err := p.store.Append(messages...); err != nil {
		p.warn("append outbox messages failed", "error", err, "count", len(messages))
	}
}

func (p *Publisher) warn(msg string, args ...any) {
	if p == nil || p.logger == nil {
		return
	}
	p.logger.Warn(msg, args...)
}

func newOutboxMessageID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
