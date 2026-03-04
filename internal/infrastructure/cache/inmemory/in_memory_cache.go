package inmemory

import (
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application"
)

var _ application.Cache = (*InMemoryCache)(nil)

type InMemoryCache struct {
	store sync.Map
}

type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
	hasExpiry bool
}

func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		store: sync.Map{},
	}
}

func (c *InMemoryCache) Get(key string) (interface{}, bool) {
	value, exists := c.store.Load(key)
	if !exists {
		return nil, false
	}
	entry, ok := value.(cacheEntry)
	if !ok {
		return nil, false
	}
	if entry.hasExpiry && time.Now().After(entry.expiresAt) {
		c.store.Delete(key)
		return nil, false
	}
	return entry.value, true
}

func (c *InMemoryCache) Set(key string, value interface{}) {
	c.store.Store(key, cacheEntry{
		value: value,
	})
}

func (c *InMemoryCache) SetWithTTL(key string, value interface{}, ttlSeconds int) {
	if ttlSeconds <= 0 {
		c.Set(key, value)
		return
	}
	c.store.Store(key, cacheEntry{
		value:     value,
		hasExpiry: true,
		expiresAt: time.Now().Add(time.Duration(ttlSeconds) * time.Second),
	})
}

func (c *InMemoryCache) Delete(key string) {
	c.store.Delete(key)
}
