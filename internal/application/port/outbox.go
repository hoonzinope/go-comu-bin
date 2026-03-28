package port

import (
	"context"
	"time"
)

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
	Append(ctx context.Context, messages ...OutboxMessage) error
}

type OutboxStore interface {
	OutboxAppender
	FetchReady(ctx context.Context, limit int, now time.Time) ([]OutboxMessage, error)
	SelectByID(ctx context.Context, id string) (*OutboxMessage, error)
	SelectDead(ctx context.Context, limit int, lastID string) ([]OutboxMessage, error)
	RenewProcessing(ctx context.Context, id string, nextAttemptAt time.Time) error
	MarkSucceeded(ctx context.Context, ids ...string) error
	MarkRetry(ctx context.Context, id string, nextAttemptAt time.Time, err string) error
	MarkDead(ctx context.Context, id string, err string) error
}

type EventSerializer interface {
	Serialize(event DomainEvent) (eventName string, payload []byte, occurredAt time.Time, err error)
	Deserialize(eventName string, payload []byte, occurredAt time.Time) (DomainEvent, error)
}
