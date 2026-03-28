package common

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
)

var DefaultEventSerializer port.EventSerializer = appevent.NewJSONEventSerializer()

func DispatchDomainActions(tx port.TxScope, dispatcher port.ActionHookDispatcher, events ...port.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}
	if tx == nil {
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
		eventName, payload, occurredAt, err := DefaultEventSerializer.Serialize(event)
		if err != nil {
			return customerror.Mark(customerror.ErrInternalServerError, fmt.Sprintf("serialize event for outbox: %v", err))
		}
		id, err := newOutboxMessageID()
		if err != nil {
			return customerror.Mark(customerror.ErrInternalServerError, fmt.Sprintf("generate outbox message id: %v", err))
		}
		if occurredAt.IsZero() {
			occurredAt = time.Now()
		}
		messages = append(messages, port.OutboxMessage{ID: id, EventName: eventName, Payload: payload, OccurredAt: occurredAt, NextAttemptAt: occurredAt, Status: port.OutboxStatusPending})
	}
	ctx := tx.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if err := outbox.Append(ctx, messages...); err != nil {
		return customerror.WrapRepository("append outbox messages", err)
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
