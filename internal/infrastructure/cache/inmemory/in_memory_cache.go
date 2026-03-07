package inmemory

import (
	"strings"
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"golang.org/x/sync/singleflight"
)

var _ port.Cache = (*InMemoryCache)(nil)

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

func (c *InMemoryCache) Get(key string) (interface{}, bool, error) {
	value, exists := c.store.Load(key)
	if !exists {
		return nil, false, nil
	}
	entry, ok := value.(cacheEntry)
	if !ok {
		return nil, false, nil
	}
	if entry.hasExpiry && time.Now().After(entry.expiresAt) {
		c.store.Delete(key)
		return nil, false, nil
	}
	return entry.value, true, nil
}

func (c *InMemoryCache) Set(key string, value interface{}) error {
	c.store.Store(key, cacheEntry{
		value: value,
	})
	return nil
}

func (c *InMemoryCache) SetWithTTL(key string, value interface{}, ttlSeconds int) error {
	if ttlSeconds <= 0 {
		return c.Set(key, value)
	}
	c.store.Store(key, cacheEntry{
		value:     value,
		hasExpiry: true,
		expiresAt: time.Now().Add(time.Duration(ttlSeconds) * time.Second),
	})
	return nil
}

func (c *InMemoryCache) Delete(key string) error {
	c.store.Delete(key)
	return nil
}

func (c *InMemoryCache) DeleteByPrefix(prefix string) (int, error) {
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
	return deleted, nil
}

func (c *InMemoryCache) GetOrSetWithTTL(key string, ttlSeconds int, loader func() (interface{}, error)) (interface{}, error) {
	if value, ok, err := c.Get(key); err != nil {
		return nil, err
	} else if ok {
		return value, nil
	}

	value, err, _ := c.sf.Do(key, func() (interface{}, error) {
		if cached, ok, getErr := c.Get(key); getErr != nil {
			return nil, getErr
		} else if ok {
			return cached, nil
		}
		loaded, loadErr := loader()
		if loadErr != nil {
			return nil, loadErr
		}
		if setErr := c.SetWithTTL(key, loaded, ttlSeconds); setErr != nil {
			return nil, setErr
		}
		return loaded, nil
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}
