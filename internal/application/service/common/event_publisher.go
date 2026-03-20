package common

import "github.com/hoonzinope/go-comu-bin/internal/application/port"

type noopActionHookDispatcher struct{}

func (noopActionHookDispatcher) Dispatch(events ...port.DomainEvent) {}

type eventPublisherActionHookDispatcher struct {
	publisher port.EventPublisher
}

func (d eventPublisherActionHookDispatcher) Dispatch(events ...port.DomainEvent) {
	if d.publisher == nil {
		return
	}
	d.publisher.Publish(events...)
}

func WrapEventPublisherAsActionDispatcher(publisher port.EventPublisher) port.ActionHookDispatcher {
	if publisher == nil {
		return nil
	}
	return eventPublisherActionHookDispatcher{publisher: publisher}
}

func ResolveActionDispatcher(dispatcher port.ActionHookDispatcher) port.ActionHookDispatcher {
	if dispatcher != nil {
		return dispatcher
	}
	return noopActionHookDispatcher{}
}
