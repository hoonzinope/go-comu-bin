package inmemory

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"golang.org/x/sync/singleflight"
)

var _ port.Cache = (*InMemoryCache)(nil)

type InMemoryCache struct {
	mu          sync.RWMutex
	store       map[string]cacheEntry
	prefixIndex map[string]map[string]struct{}
	sf          singleflight.Group
}

type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
	hasExpiry bool
}

func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		store:       make(map[string]cacheEntry),
		prefixIndex: make(map[string]map[string]struct{}),
	}
}

func (c *InMemoryCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	_ = ctx
	c.mu.RLock()
	entry, exists := c.store[key]
	c.mu.RUnlock()
	if !exists {
		return nil, false, nil
	}
	if entry.hasExpiry && time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		if current, ok := c.store[key]; ok && current.hasExpiry && time.Now().After(current.expiresAt) {
			c.deleteLocked(key)
		}
		c.mu.Unlock()
		return nil, false, nil
	}
	return entry.value, true, nil
}

func (c *InMemoryCache) Set(ctx context.Context, key string, value interface{}) error {
	_ = ctx
	c.mu.Lock()
	c.setLocked(key, cacheEntry{value: value})
	c.mu.Unlock()
	return nil
}

func (c *InMemoryCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttlSeconds int) error {
	if ttlSeconds <= 0 {
		return c.Set(ctx, key, value)
	}
	c.mu.Lock()
	c.setLocked(key, cacheEntry{
		value:     value,
		hasExpiry: true,
		expiresAt: time.Now().Add(time.Duration(ttlSeconds) * time.Second),
	})
	c.mu.Unlock()
	return nil
}

func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	_ = ctx
	c.mu.Lock()
	c.deleteLocked(key)
	c.mu.Unlock()
	return nil
}

func (c *InMemoryCache) DeleteByPrefix(ctx context.Context, prefix string) (int, error) {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()

	if strings.HasSuffix(prefix, ":") {
		keys := c.keysForIndexedPrefixLocked(prefix)
		for _, key := range keys {
			c.deleteLocked(key)
		}
		return len(keys), nil
	}

	deleted := 0
	for key := range c.store {
		if strings.HasPrefix(key, prefix) {
			c.deleteLocked(key)
			deleted++
		}
	}
	return deleted, nil
}

func (c *InMemoryCache) ExistsByPrefix(ctx context.Context, prefix string) (bool, error) {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()

	if strings.HasSuffix(prefix, ":") {
		for _, key := range c.keysForIndexedPrefixLocked(prefix) {
			entry, exists := c.store[key]
			if !exists {
				continue
			}
			if entry.hasExpiry && time.Now().After(entry.expiresAt) {
				c.deleteLocked(key)
				continue
			}
			return true, nil
		}
		return false, nil
	}

	for key, entry := range c.store {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if entry.hasExpiry && time.Now().After(entry.expiresAt) {
			c.deleteLocked(key)
			continue
		}
		return true, nil
	}
	return false, nil
}

func (c *InMemoryCache) GetOrSetWithTTL(ctx context.Context, key string, ttlSeconds int, loader func(context.Context) (interface{}, error)) (interface{}, error) {
	if value, ok, err := c.Get(ctx, key); err != nil {
		return nil, err
	} else if ok {
		return value, nil
	}

	value, err, _ := c.sf.Do(key, func() (interface{}, error) {
		if cached, ok, getErr := c.Get(ctx, key); getErr != nil {
			return nil, getErr
		} else if ok {
			return cached, nil
		}
		loaded, loadErr := loader(ctx)
		if loadErr != nil {
			return nil, loadErr
		}
		if setErr := c.SetWithTTL(ctx, key, loaded, ttlSeconds); setErr != nil {
			return nil, setErr
		}
		return loaded, nil
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (c *InMemoryCache) setLocked(key string, entry cacheEntry) {
	if _, exists := c.store[key]; !exists {
		c.addToPrefixIndexLocked(key)
	}
	c.store[key] = entry
}

func (c *InMemoryCache) deleteLocked(key string) {
	if _, exists := c.store[key]; !exists {
		return
	}
	delete(c.store, key)
	c.removeFromPrefixIndexLocked(key)
}

func (c *InMemoryCache) addToPrefixIndexLocked(key string) {
	for _, prefix := range cachePrefixes(key) {
		keys := c.prefixIndex[prefix]
		if keys == nil {
			keys = make(map[string]struct{})
			c.prefixIndex[prefix] = keys
		}
		keys[key] = struct{}{}
	}
}

func (c *InMemoryCache) removeFromPrefixIndexLocked(key string) {
	for _, prefix := range cachePrefixes(key) {
		keys := c.prefixIndex[prefix]
		if keys == nil {
			continue
		}
		delete(keys, key)
		if len(keys) == 0 {
			delete(c.prefixIndex, prefix)
		}
	}
}

func (c *InMemoryCache) keysForIndexedPrefixLocked(prefix string) []string {
	keys := c.prefixIndex[prefix]
	if len(keys) == 0 {
		return nil
	}
	out := make([]string, 0, len(keys))
	for key := range keys {
		out = append(out, key)
	}
	return out
}

func cachePrefixes(key string) []string {
	prefixes := make([]string, 0, strings.Count(key, ":"))
	for idx, r := range key {
		if r == ':' {
			prefixes = append(prefixes, key[:idx+1])
		}
	}
	return prefixes
}
