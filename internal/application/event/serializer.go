package event

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.EventSerializer = (*JSONEventSerializer)(nil)

type JSONEventSerializer struct{}

func NewJSONEventSerializer() *JSONEventSerializer {
	return &JSONEventSerializer{}
}

func (s *JSONEventSerializer) Serialize(event port.DomainEvent) (string, []byte, time.Time, error) {
	if event == nil {
		return "", nil, time.Time{}, fmt.Errorf("event is nil")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return "", nil, time.Time{}, err
	}
	return event.EventName(), payload, event.OccurredAt(), nil
}

func (s *JSONEventSerializer) Deserialize(eventName string, payload []byte, occurredAt time.Time) (port.DomainEvent, error) {
	switch eventName {
	case EventNameBoardChanged:
		return deserializeEvent(payload, occurredAt, func(e BoardChanged) time.Time { return e.At }, func(e *BoardChanged, at time.Time) { e.At = at })
	case EventNamePostChanged:
		return deserializeEvent(payload, occurredAt, func(e PostChanged) time.Time { return e.At }, func(e *PostChanged, at time.Time) { e.At = at })
	case EventNameCommentChanged:
		return deserializeEvent(payload, occurredAt, func(e CommentChanged) time.Time { return e.At }, func(e *CommentChanged, at time.Time) { e.At = at })
	case EventNameReactionChanged:
		return deserializeEvent(payload, occurredAt, func(e ReactionChanged) time.Time { return e.At }, func(e *ReactionChanged, at time.Time) { e.At = at })
	case EventNameAttachmentChanged:
		return deserializeEvent(payload, occurredAt, func(e AttachmentChanged) time.Time { return e.At }, func(e *AttachmentChanged, at time.Time) { e.At = at })
	case EventNameReportChanged:
		return deserializeEvent(payload, occurredAt, func(e ReportChanged) time.Time { return e.At }, func(e *ReportChanged, at time.Time) { e.At = at })
	case EventNameNotificationTriggered:
		return deserializeEvent(payload, occurredAt, func(e NotificationTriggered) time.Time { return e.At }, func(e *NotificationTriggered, at time.Time) { e.At = at })
	case EventNameSignupEmailVerificationRequested:
		return deserializeEvent(payload, occurredAt, func(e SignupEmailVerificationRequested) time.Time { return e.At }, func(e *SignupEmailVerificationRequested, at time.Time) { e.At = at })
	case EventNameEmailVerificationResendRequested:
		return deserializeEvent(payload, occurredAt, func(e EmailVerificationResendRequested) time.Time { return e.At }, func(e *EmailVerificationResendRequested, at time.Time) { e.At = at })
	case EventNamePasswordResetRequested:
		return deserializeEvent(payload, occurredAt, func(e PasswordResetRequested) time.Time { return e.At }, func(e *PasswordResetRequested, at time.Time) { e.At = at })
	default:
		return nil, fmt.Errorf("unsupported event name: %s", eventName)
	}
}

func deserializeEvent[T port.DomainEvent](payload []byte, occurredAt time.Time, getOccurredAt func(T) time.Time, setOccurredAt func(*T, time.Time)) (T, error) {
	var event T
	if err := json.Unmarshal(payload, &event); err != nil {
		return event, err
	}
	if getOccurredAt(event).IsZero() {
		setOccurredAt(&event, occurredAt)
	}
	return event, nil
}
