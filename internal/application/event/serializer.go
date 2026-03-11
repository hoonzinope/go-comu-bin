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
		var event BoardChanged
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		if event.At.IsZero() {
			event.At = occurredAt
		}
		return event, nil
	case EventNamePostChanged:
		var event PostChanged
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		if event.At.IsZero() {
			event.At = occurredAt
		}
		return event, nil
	case EventNameCommentChanged:
		var event CommentChanged
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		if event.At.IsZero() {
			event.At = occurredAt
		}
		return event, nil
	case EventNameReactionChanged:
		var event ReactionChanged
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		if event.At.IsZero() {
			event.At = occurredAt
		}
		return event, nil
	case EventNameAttachmentChanged:
		var event AttachmentChanged
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		if event.At.IsZero() {
			event.At = occurredAt
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported event name: %s", eventName)
	}
}
