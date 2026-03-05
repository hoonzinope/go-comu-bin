package inmemory

import (
	"strings"
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application"
	"golang.org/x/sync/singleflight"
)

var _ application.Cache = (*InMemoryCache)(nil)

type InMemoryCache struct {
	store sync.Map
	sf    singleflight.Group
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

func (c *InMemoryCache) DeleteByPrefix(prefix string) int {
	deleted := 0
	c.store.Range(func(key, _ interface{}) bool {
		k, ok := key.(string)
		if !ok {
			return true
		}
		if strings.HasPrefix(k, prefix) {
			c.store.Delete(k)
			deleted++
		}
		return true
	})
	return deleted
}

func (c *InMemoryCache) GetOrSetWithTTL(key string, ttlSeconds int, loader func() (interface{}, error)) (interface{}, error) {
	if value, ok := c.Get(key); ok {
		return value, nil
	}

	value, err, _ := c.sf.Do(key, func() (interface{}, error) {
		if cached, ok := c.Get(key); ok {
			return cached, nil
		}
		loaded, loadErr := loader()
		if loadErr != nil {
			return nil, loadErr
		}
		c.SetWithTTL(key, loaded, ttlSeconds)
		return loaded, nil
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}
