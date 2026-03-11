package service

import "github.com/hoonzinope/go-comu-bin/internal/application/port"

type noopEventPublisher struct{}

func (noopEventPublisher) Publish(events ...port.DomainEvent) {}

func resolveEventPublisher(publisher port.EventPublisher) port.EventPublisher {
	if publisher != nil {
		return publisher
	}
	return noopEventPublisher{}
}
