package auth

import (
	"context"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	cacheInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingCache struct {
	lastCtx context.Context
}

func (c *recordingCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	c.lastCtx = ctx
	return 1, true, nil
}

func (c *recordingCache) Set(ctx context.Context, key string, value interface{}) error {
	c.lastCtx = ctx
	return nil
}

func (c *recordingCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttlSeconds int) error {
	c.lastCtx = ctx
	return nil
}

func (c *recordingCache) Delete(ctx context.Context, key string) error {
	c.lastCtx = ctx
	return nil
}

func (c *recordingCache) DeleteByPrefix(ctx context.Context, prefix string) (int, error) {
	c.lastCtx = ctx
	return 0, nil
}

func (c *recordingCache) ExistsByPrefix(ctx context.Context, prefix string) (bool, error) {
	c.lastCtx = ctx
	return true, nil
}

func (c *recordingCache) GetOrSetWithTTL(ctx context.Context, key string, ttlSeconds int, loader func(context.Context) (interface{}, error)) (interface{}, error) {
	c.lastCtx = ctx
	return loader(ctx)
}

func TestCacheSessionRepository_ForwardsContextToCache(t *testing.T) {
	cache := &recordingCache{}
	repo := NewCacheSessionRepository(cache)
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("request_id"), "req-1")

	require.NoError(t, repo.Save(ctx, 1, "token", 60))
	assert.Same(t, ctx, cache.lastCtx)

	_, err := repo.Exists(ctx, 1, "token")
	require.NoError(t, err)
	assert.Same(t, ctx, cache.lastCtx)

	require.NoError(t, repo.DeleteByUser(ctx, 1))
	assert.Same(t, ctx, cache.lastCtx)

	_, err = repo.ExistsByUser(ctx, 1)
	require.NoError(t, err)
	assert.Same(t, ctx, cache.lastCtx)
}

func TestCacheSessionRepository_WithUserLockSerializesWrites(t *testing.T) {
	cache := cacheInMemory.NewInMemoryCache()
	repo := NewCacheSessionRepository(cache)
	ctx := context.Background()

	entered := make(chan struct{})
	release := make(chan struct{})
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- repo.WithUserLock(ctx, 7, func(port.SessionRepositoryScope) error {
			close(entered)
			<-release
			return nil
		})
	}()

	select {
	case err := <-lockDone:
		require.NoError(t, err)
		t.Fatal("lock scope returned before release")
	case <-entered:
	}

	saveDone := make(chan error, 1)
	go func() {
		saveDone <- repo.Save(ctx, 7, "token", 60)
	}()

	select {
	case err := <-saveDone:
		t.Fatalf("save completed while user lock was held: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-lockDone)
	require.NoError(t, <-saveDone)
}
