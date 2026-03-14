package port

import "time"

type OutboxStatus string

const (
	OutboxStatusPending    OutboxStatus = "pending"
	OutboxStatusProcessing OutboxStatus = "processing"
	OutboxStatusDead       OutboxStatus = "dead"
)

type OutboxMessage struct {
	ID            string
	EventName     string
	Payload       []byte
	OccurredAt    time.Time
	AttemptCount  int
	NextAttemptAt time.Time
	Status        OutboxStatus
	LastError     string
}

type OutboxAppender interface {
	Append(messages ...OutboxMessage) error
}

type OutboxStore interface {
	OutboxAppender
	FetchReady(limit int, now time.Time) ([]OutboxMessage, error)
	SelectByID(id string) (*OutboxMessage, error)
	SelectDead(limit int, lastID string) ([]OutboxMessage, error)
	MarkSucceeded(ids ...string) error
	MarkRetry(id string, nextAttemptAt time.Time, err string) error
	MarkDead(id string, err string) error
}

type EventSerializer interface {
	Serialize(event DomainEvent) (eventName string, payload []byte, occurredAt time.Time, err error)
	Deserialize(eventName string, payload []byte, occurredAt time.Time) (DomainEvent, error)
}
