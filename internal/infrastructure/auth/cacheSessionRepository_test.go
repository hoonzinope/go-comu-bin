package auth

import (
	"context"
	"testing"

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
}
