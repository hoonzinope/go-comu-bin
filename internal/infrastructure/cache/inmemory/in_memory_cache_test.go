package inmemory

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryCache_DeleteByPrefix(t *testing.T) {
	cache := NewInMemoryCache()
	ctx := context.Background()
	require.NoError(t, cache.Set(ctx, "posts:1", "a"))
	require.NoError(t, cache.Set(ctx, "posts:2", "b"))
	require.NoError(t, cache.Set(ctx, "comments:1", "c"))

	deleted, err := cache.DeleteByPrefix(ctx, "posts:")
	require.NoError(t, err)
	assert.Equal(t, 2, deleted)

	_, ok, err := cache.Get(ctx, "posts:1")
	require.NoError(t, err)
	assert.False(t, ok)
	_, ok, err = cache.Get(ctx, "posts:2")
	require.NoError(t, err)
	assert.False(t, ok)
	v, ok, err := cache.Get(ctx, "comments:1")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "c", v)
}

func TestInMemoryCache_GetOrSetWithTTL_HitAndMiss(t *testing.T) {
	cache := NewInMemoryCache()
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

func TestInMemoryCache_GetOrSetWithTTL_ConcurrentSingleflight(t *testing.T) {
	cache := NewInMemoryCache()
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

func TestInMemoryCache_GetOrSetWithTTL_PassesContextToLoader(t *testing.T) {
	cache := NewInMemoryCache()
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("request_id"), "req-1")

	value, err := cache.GetOrSetWithTTL(ctx, "boards:list", 30, func(loaderCtx context.Context) (interface{}, error) {
		assert.Same(t, ctx, loaderCtx)
		return loaderCtx.Value(contextKey("request_id")), nil
	})

	require.NoError(t, err)
	assert.Equal(t, "req-1", value)
}
