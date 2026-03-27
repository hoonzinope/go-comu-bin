package ristretto

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	ristrettolib "github.com/dgraph-io/ristretto/v2"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"golang.org/x/sync/singleflight"
)

var _ port.Cache = (*Cache)(nil)

type Config struct {
	NumCounters int64
	MaxCost     int64
	BufferItems int64
	Metrics     bool
}

type Cache struct {
	mu          sync.RWMutex
	keys        map[string]struct{}
	prefixIndex map[string]map[string]struct{}
	cache       *ristrettolib.Cache[string, cacheValue]
	sf          singleflight.Group
	setFn       func(key string, value cacheValue, cost int64, ttl time.Duration) bool
}

type cacheValue struct {
	key   string
	value interface{}
}

func NewCache(cfg Config) (*Cache, error) {
	if cfg.NumCounters <= 0 {
		return nil, fmt.Errorf("invalid ristretto cache numCounters: %d (must be > 0)", cfg.NumCounters)
	}
	if cfg.MaxCost <= 0 {
		return nil, fmt.Errorf("invalid ristretto cache maxCost: %d (must be > 0)", cfg.MaxCost)
	}
	if cfg.BufferItems <= 0 {
		return nil, fmt.Errorf("invalid ristretto cache bufferItems: %d (must be > 0)", cfg.BufferItems)
	}

	c := &Cache{
		keys:        make(map[string]struct{}),
		prefixIndex: make(map[string]map[string]struct{}),
	}

	underlying, err := ristrettolib.NewCache(&ristrettolib.Config[string, cacheValue]{
		NumCounters: cfg.NumCounters,
		MaxCost:     cfg.MaxCost,
		BufferItems: cfg.BufferItems,
		Metrics:     cfg.Metrics,
		OnEvict: func(item *ristrettolib.Item[cacheValue]) {
			if item == nil {
				return
			}
			c.removeKeyFromIndexes(item.Value.key)
		},
	})
	if err != nil {
		return nil, err
	}

	c.cache = underlying
	c.setFn = func(key string, value cacheValue, cost int64, ttl time.Duration) bool {
		if ttl <= 0 {
			return c.cache.Set(key, value, cost)
		}
		return c.cache.SetWithTTL(key, value, cost, ttl)
	}
	return c, nil
}

func (c *Cache) Close() {
	if c == nil || c.cache == nil {
		return
	}
	c.cache.Wait()
	c.cache.Close()
}

func (c *Cache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	_ = ctx
	value, ok := c.cache.Get(key)
	if !ok {
		c.removeKeyFromIndexes(key)
		return nil, false, nil
	}
	return value.value, true, nil
}

func (c *Cache) Set(ctx context.Context, key string, value interface{}) error {
	return c.set(ctx, key, value, 0)
}

func (c *Cache) SetWithTTL(ctx context.Context, key string, value interface{}, ttlSeconds int) error {
	if ttlSeconds <= 0 {
		return c.Set(ctx, key, value)
	}
	return c.set(ctx, key, value, time.Duration(ttlSeconds)*time.Second)
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	_ = ctx
	c.cache.Del(key)
	c.removeKeyFromIndexes(key)
	return nil
}

func (c *Cache) DeleteByPrefix(ctx context.Context, prefix string) (int, error) {
	_ = ctx
	keys := c.keysForPrefix(prefix)
	deleted := 0
	for _, key := range keys {
		if _, ok := c.cache.Get(key); ok {
			c.cache.Del(key)
			deleted++
		}
		c.removeKeyFromIndexes(key)
	}
	return deleted, nil
}

func (c *Cache) ExistsByPrefix(ctx context.Context, prefix string) (bool, error) {
	_ = ctx
	keys := c.keysForPrefix(prefix)
	for _, key := range keys {
		if _, ok := c.cache.Get(key); ok {
			return true, nil
		}
		c.removeKeyFromIndexes(key)
	}
	return false, nil
}

func (c *Cache) GetOrSetWithTTL(ctx context.Context, key string, ttlSeconds int, loader func(context.Context) (interface{}, error)) (interface{}, error) {
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

func (c *Cache) set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	_ = ctx
	cachedValue := cacheValue{key: key, value: value}
	if ok := c.setFn(key, cachedValue, 1, ttl); !ok {
		return fmt.Errorf("cache set %q rejected by ristretto", key)
	}
	c.cache.Wait()
	c.addKeyToIndexes(key)
	return nil
}

func (c *Cache) keysForPrefix(prefix string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if strings.HasSuffix(prefix, ":") {
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

	out := make([]string, 0, len(c.keys))
	for key := range c.keys {
		if strings.HasPrefix(key, prefix) {
			out = append(out, key)
		}
	}
	return out
}

func (c *Cache) addKeyToIndexes(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.keys[key]; !exists {
		c.keys[key] = struct{}{}
	}
	for _, prefix := range cachePrefixes(key) {
		keys := c.prefixIndex[prefix]
		if keys == nil {
			keys = make(map[string]struct{})
			c.prefixIndex[prefix] = keys
		}
		keys[key] = struct{}{}
	}
}

func (c *Cache) removeKeyFromIndexes(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.keys, key)
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

func cachePrefixes(key string) []string {
	prefixes := make([]string, 0, strings.Count(key, ":"))
	for idx, r := range key {
		if r == ':' {
			prefixes = append(prefixes, key[:idx+1])
		}
	}
	return prefixes
}
