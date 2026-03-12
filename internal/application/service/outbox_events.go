package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

var defaultEventSerializer port.EventSerializer = appevent.NewJSONEventSerializer()

func appendEventsToOutbox(tx port.TxScope, events ...port.DomainEvent) error {
	return dispatchDomainActions(tx, nil, events...)
}

func dispatchDomainActions(tx port.TxScope, dispatcher port.ActionHookDispatcher, events ...port.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}
	if tx == nil || len(events) == 0 {
		if dispatcher != nil {
			dispatcher.Dispatch(events...)
		}
		return nil
	}
	outbox := tx.Outbox()
	if outbox == nil {
		if dispatcher != nil {
			dispatcher.Dispatch(events...)
		}
		return nil
	}
	messages := make([]port.OutboxMessage, 0, len(events))
	for _, event := range events {
		eventName, payload, occurredAt, err := defaultEventSerializer.Serialize(event)
		if err != nil {
			return customError.Mark(customError.ErrInternalServerError, fmt.Sprintf("serialize event for outbox: %v", err))
		}
		id, err := newOutboxMessageID()
		if err != nil {
			return customError.Mark(customError.ErrInternalServerError, fmt.Sprintf("generate outbox message id: %v", err))
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
	if err := outbox.Append(messages...); err != nil {
		return customError.WrapRepository("append outbox messages", err)
	}
	return nil
}

func newOutboxMessageID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
