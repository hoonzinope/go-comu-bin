package service

import "github.com/hoonzinope/go-comu-bin/internal/application"

const (
	listCacheTTLSeconds   = 30
	detailCacheTTLSeconds = 30
)

type noopCache struct{}

func (n *noopCache) Get(key string) (interface{}, bool) { return nil, false }
func (n *noopCache) Set(key string, value interface{})  {}
func (n *noopCache) SetWithTTL(key string, value interface{}, ttlSeconds int) {
}
func (n *noopCache) Delete(key string)                {}
func (n *noopCache) DeleteByPrefix(prefix string) int { return 0 }
func (n *noopCache) GetOrSetWithTTL(key string, ttlSeconds int, loader func() (interface{}, error)) (interface{}, error) {
	return loader()
}

func resolveCache(caches []application.Cache) application.Cache {
	if len(caches) > 0 && caches[0] != nil {
		return caches[0]
	}
	return &noopCache{}
}
