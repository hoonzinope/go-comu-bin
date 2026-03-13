package testutil

import (
	"context"
	"strings"
	"sync"
)

type SpyCache struct {
	mu         sync.Mutex
	store      map[string]interface{}
	loadCounts map[string]int
}

func NewSpyCache() *SpyCache {
	return &SpyCache{
		store:      make(map[string]interface{}),
		loadCounts: make(map[string]int),
	}
}

func (c *SpyCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.store[key]
	return v, ok, nil
}

func (c *SpyCache) Set(ctx context.Context, key string, value interface{}) error {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
	return nil
}

func (c *SpyCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttlSeconds int) error {
	_ = ttlSeconds
	return c.Set(ctx, key, value)
}

func (c *SpyCache) Delete(ctx context.Context, key string) error {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
	return nil
}

func (c *SpyCache) DeleteByPrefix(ctx context.Context, prefix string) (int, error) {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	deleted := 0
	for k := range c.store {
		if strings.HasPrefix(k, prefix) {
			delete(c.store, k)
			deleted++
		}
	}
	return deleted, nil
}

func (c *SpyCache) GetOrSetWithTTL(ctx context.Context, key string, ttlSeconds int, loader func(context.Context) (interface{}, error)) (interface{}, error) {
	if v, ok, err := c.Get(ctx, key); err != nil {
		return nil, err
	} else if ok {
		return v, nil
	}
	v, err := loader(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.loadCounts[key]++
	c.mu.Unlock()
	if err := c.SetWithTTL(ctx, key, v, ttlSeconds); err != nil {
		return nil, err
	}
	return v, nil
}

func (c *SpyCache) LoadCount(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loadCounts[key]
}
