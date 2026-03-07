package noop

import "github.com/hoonzinope/go-comu-bin/internal/application/port"

var _ port.Cache = (*NoopCache)(nil)

type NoopCache struct{}

func NewNoopCache() *NoopCache {
	return &NoopCache{}
}

func (n *NoopCache) Get(key string) (interface{}, bool, error) { return nil, false, nil }
func (n *NoopCache) Set(key string, value interface{}) error   { return nil }
func (n *NoopCache) SetWithTTL(key string, value interface{}, ttlSeconds int) error {
	return nil
}
func (n *NoopCache) Delete(key string) error { return nil }
func (n *NoopCache) DeleteByPrefix(prefix string) (int, error) {
	return 0, nil
}
func (n *NoopCache) GetOrSetWithTTL(key string, ttlSeconds int, loader func() (interface{}, error)) (interface{}, error) {
	return loader()
}
