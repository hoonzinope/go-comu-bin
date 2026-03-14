package model

import "time"

type OutboxDeadMessage struct {
	ID            string
	EventName     string
	AttemptCount  int
	LastError     string
	OccurredAt    time.Time
	NextAttemptAt time.Time
}

type OutboxDeadMessageList struct {
	Messages   []OutboxDeadMessage
	Limit      int
	LastID     string
	HasMore    bool
	NextLastID *string
}
