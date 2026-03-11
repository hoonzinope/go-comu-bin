package port

import "time"

type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
}

type EventHandler interface {
	Handle(event DomainEvent) error
}

type EventBus interface {
	Subscribe(eventName string, handler EventHandler)
	Publish(events ...DomainEvent)
}

type EventPublisher interface {
	Publish(events ...DomainEvent)
}
