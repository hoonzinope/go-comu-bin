package testutil

import (
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

func (c *SpyCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.store[key]
	return v, ok
}

func (c *SpyCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
}

func (c *SpyCache) SetWithTTL(key string, value interface{}, ttlSeconds int) {
	c.Set(key, value)
}

func (c *SpyCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
}

func (c *SpyCache) DeleteByPrefix(prefix string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	deleted := 0
	for k := range c.store {
		if strings.HasPrefix(k, prefix) {
			delete(c.store, k)
			deleted++
		}
	}
	return deleted
}

func (c *SpyCache) GetOrSetWithTTL(key string, ttlSeconds int, loader func() (interface{}, error)) (interface{}, error) {
	if v, ok := c.Get(key); ok {
		return v, nil
	}
	v, err := loader()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.loadCounts[key]++
	c.mu.Unlock()
	c.SetWithTTL(key, v, ttlSeconds)
	return v, nil
}

func (c *SpyCache) LoadCount(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loadCounts[key]
}
