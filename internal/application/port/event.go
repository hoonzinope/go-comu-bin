package port

import (
	"context"
	"time"
)

type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
}

type EventHandler interface {
	Handle(ctx context.Context, event DomainEvent) error
}

type EventPublisher interface {
	Publish(events ...DomainEvent)
}
