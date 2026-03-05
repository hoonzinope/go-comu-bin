package noop

import "github.com/hoonzinope/go-comu-bin/internal/application"

var _ application.Cache = (*NoopCache)(nil)

type NoopCache struct{}

func NewNoopCache() *NoopCache {
	return &NoopCache{}
}

func (n *NoopCache) Get(key string) (interface{}, bool) { return nil, false }
func (n *NoopCache) Set(key string, value interface{})  {}
func (n *NoopCache) SetWithTTL(key string, value interface{}, ttlSeconds int) {
}
func (n *NoopCache) Delete(key string)                {}
func (n *NoopCache) DeleteByPrefix(prefix string) int { return 0 }
func (n *NoopCache) GetOrSetWithTTL(key string, ttlSeconds int, loader func() (interface{}, error)) (interface{}, error) {
	return loader()
}
