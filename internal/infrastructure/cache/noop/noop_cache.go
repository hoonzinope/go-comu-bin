package noop

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.Cache = (*NoopCache)(nil)

type NoopCache struct{}

func NewNoopCache() *NoopCache {
	return &NoopCache{}
}

func (n *NoopCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	_ = ctx
	_ = key
	return nil, false, nil
}

func (n *NoopCache) Set(ctx context.Context, key string, value interface{}) error {
	_ = ctx
	_ = key
	_ = value
	return nil
}

func (n *NoopCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttlSeconds int) error {
	_ = ctx
	_ = key
	_ = value
	_ = ttlSeconds
	return nil
}

func (n *NoopCache) Delete(ctx context.Context, key string) error {
	_ = ctx
	_ = key
	return nil
}

func (n *NoopCache) DeleteByPrefix(ctx context.Context, prefix string) (int, error) {
	_ = ctx
	_ = prefix
	return 0, nil
}

func (n *NoopCache) GetOrSetWithTTL(ctx context.Context, key string, ttlSeconds int, loader func(context.Context) (interface{}, error)) (interface{}, error) {
	_ = key
	_ = ttlSeconds
	return loader(ctx)
}
