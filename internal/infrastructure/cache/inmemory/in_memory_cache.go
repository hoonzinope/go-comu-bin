package inmemory

import (
	"sync"

	"github.com/hoonzinope/go-comu-bin/internal/application"
)

var _ application.Cache = (*InMemoryCache)(nil)

type InMemoryCache struct {
	store sync.Map
}

func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		store: sync.Map{},
	}
}

func (c *InMemoryCache) Get(key string) (interface{}, bool) {
	value, exists := c.store.Load(key)
	return value, exists
}

func (c *InMemoryCache) Set(key string, value interface{}) {
	c.store.Store(key, value)
}

func (c *InMemoryCache) SetWithTTL(key string, value interface{}, ttlSeconds int) {
	c.store.Store(key, value)

}

func (c *InMemoryCache) Delete(key string) {
	c.store.Delete(key)
}
