package ristretto

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache_GetSetAndTTL(t *testing.T) {
	cache := mustNewTestCache(t)
	ctx := context.Background()

	require.NoError(t, cache.Set(ctx, "posts:1", "value-1"))

	v, ok, err := cache.Get(ctx, "posts:1")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "value-1", v)

	require.NoError(t, cache.SetWithTTL(ctx, "posts:ttl", "value-ttl", 1))

	v, ok, err = cache.Get(ctx, "posts:ttl")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "value-ttl", v)

	time.Sleep(1100 * time.Millisecond)

	v, ok, err = cache.Get(ctx, "posts:ttl")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, v)
}

func TestCache_DeleteByPrefix_UsesPrefixIndex(t *testing.T) {
	cache := mustNewTestCache(t)
	ctx := context.Background()

	require.NoError(t, cache.Set(ctx, "posts:list:board:1:limit:10:last:0", "a"))
	require.NoError(t, cache.Set(ctx, "posts:list:board:1:limit:10:last:10", "b"))
	require.NoError(t, cache.Set(ctx, "posts:list:board:2:limit:10:last:0", "c"))

	cache.mu.RLock()
	keysForPrefix := len(cache.prefixIndex["posts:list:board:1:"])
	cache.mu.RUnlock()
	assert.Equal(t, 2, keysForPrefix)

	deleted, err := cache.DeleteByPrefix(ctx, "posts:list:board:1:")
	require.NoError(t, err)
	assert.Equal(t, 2, deleted)

	cache.mu.RLock()
	_, exists := cache.prefixIndex["posts:list:board:1:"]
	cache.mu.RUnlock()
	assert.False(t, exists)

	_, ok, err := cache.Get(ctx, "posts:list:board:2:limit:10:last:0")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCache_ExistsByPrefix_UsesPrefixIndex(t *testing.T) {
	cache := mustNewTestCache(t)
	ctx := context.Background()

	require.NoError(t, cache.Set(ctx, "session:user:1:token-a", "a"))
	require.NoError(t, cache.Set(ctx, "session:user:1:token-b", "b"))
	require.NoError(t, cache.Set(ctx, "session:user:2:token-c", "c"))

	exists, err := cache.ExistsByPrefix(ctx, "session:user:1:")
	require.NoError(t, err)
	assert.True(t, exists)

	cache.mu.RLock()
	keysForPrefix := len(cache.prefixIndex["session:user:1:"])
	cache.mu.RUnlock()
	assert.Equal(t, 2, keysForPrefix)

	deleted, err := cache.DeleteByPrefix(ctx, "session:user:1:")
	require.NoError(t, err)
	assert.Equal(t, 2, deleted)

	exists, err = cache.ExistsByPrefix(ctx, "session:user:1:")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestCache_GetOrSetWithTTL_HitAndMiss(t *testing.T) {
	cache := mustNewTestCache(t)
	ctx := context.Background()
	var calls int32

	loader := func(context.Context) (interface{}, error) {
		atomic.AddInt32(&calls, 1)
		return "payload", nil
	}

	v, err := cache.GetOrSetWithTTL(ctx, "boards:list", 30, loader)
	require.NoError(t, err)
	assert.Equal(t, "payload", v)

	v, err = cache.GetOrSetWithTTL(ctx, "boards:list", 30, loader)
	require.NoError(t, err)
	assert.Equal(t, "payload", v)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestCache_GetOrSetWithTTL_ConcurrentSingleflight(t *testing.T) {
	cache := mustNewTestCache(t)
	ctx := context.Background()
	var calls int32
	loader := func(context.Context) (interface{}, error) {
		atomic.AddInt32(&calls, 1)
		return "ok", nil
	}

	const n = 20
	wg := sync.WaitGroup{}
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			v, err := cache.GetOrSetWithTTL(ctx, "posts:list", 30, loader)
			require.NoError(t, err)
			assert.Equal(t, "ok", v)
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestCache_GetOrSetWithTTL_PassesContextToLoader(t *testing.T) {
	cache := mustNewTestCache(t)
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("request_id"), "req-1")

	value, err := cache.GetOrSetWithTTL(ctx, "boards:list", 30, func(loaderCtx context.Context) (interface{}, error) {
		assert.Same(t, ctx, loaderCtx)
		return loaderCtx.Value(contextKey("request_id")), nil
	})

	require.NoError(t, err)
	assert.Equal(t, "req-1", value)
}

func TestCache_Set_ReturnsErrorWhenRejected(t *testing.T) {
	cache := mustNewTestCache(t)
	cache.setFn = func(string, cacheValue, int64, time.Duration) bool {
		return false
	}

	err := cache.Set(context.Background(), "posts:rejected", "value")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rejected by ristretto")
}

func mustNewTestCache(t *testing.T) *Cache {
	t.Helper()

	cache, err := NewCache(Config{
		NumCounters: 1000,
		MaxCost:     1000,
		BufferItems: 64,
		Metrics:     false,
	})
	require.NoError(t, err)
	t.Cleanup(cache.Close)
	return cache
}
