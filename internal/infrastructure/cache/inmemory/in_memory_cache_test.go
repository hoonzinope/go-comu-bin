package inmemory

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryCache_DeleteByPrefix(t *testing.T) {
	cache := NewInMemoryCache()
	require.NoError(t, cache.Set("posts:1", "a"))
	require.NoError(t, cache.Set("posts:2", "b"))
	require.NoError(t, cache.Set("comments:1", "c"))

	deleted, err := cache.DeleteByPrefix("posts:")
	require.NoError(t, err)
	assert.Equal(t, 2, deleted)

	_, ok, err := cache.Get("posts:1")
	require.NoError(t, err)
	assert.False(t, ok)
	_, ok, err = cache.Get("posts:2")
	require.NoError(t, err)
	assert.False(t, ok)
	v, ok, err := cache.Get("comments:1")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "c", v)
}

func TestInMemoryCache_GetOrSetWithTTL_HitAndMiss(t *testing.T) {
	cache := NewInMemoryCache()
	var calls int32

	loader := func() (interface{}, error) {
		atomic.AddInt32(&calls, 1)
		return "payload", nil
	}

	v, err := cache.GetOrSetWithTTL("boards:list", 30, loader)
	require.NoError(t, err)
	assert.Equal(t, "payload", v)

	v, err = cache.GetOrSetWithTTL("boards:list", 30, loader)
	require.NoError(t, err)
	assert.Equal(t, "payload", v)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestInMemoryCache_GetOrSetWithTTL_ConcurrentSingleflight(t *testing.T) {
	cache := NewInMemoryCache()
	var calls int32
	loader := func() (interface{}, error) {
		atomic.AddInt32(&calls, 1)
		return "ok", nil
	}

	const n = 20
	wg := sync.WaitGroup{}
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			v, err := cache.GetOrSetWithTTL("posts:list", 30, loader)
			require.NoError(t, err)
			assert.Equal(t, "ok", v)
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}
