package event

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/testutil"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheInvalidationHandler_BoardChanged(t *testing.T) {
	cache := testutil.NewSpyCache()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h := NewCacheInvalidationHandler(cache, logger)

	require.NoError(t, cache.Set(context.Background(), key.BoardList(10, 0), "cached"))
	require.NoError(t, cache.Set(context.Background(), key.PostList(1, 10, 0), "cached"))
	require.NoError(t, h.Handle(context.Background(), NewBoardChanged("updated", 1)))

	for _, cacheKey := range []string{
		key.BoardList(10, 0),
		key.PostList(1, 10, 0),
	} {
		_, ok, err := cache.Get(context.Background(), cacheKey)
		require.NoError(t, err)
		assert.False(t, ok)
	}
}

func TestCacheInvalidationHandler_BoardVisibilityChanged_InvalidatesVisibilitySensitiveCaches(t *testing.T) {
	cache := testutil.NewSpyCache()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h := NewCacheInvalidationHandler(cache, logger)

	require.NoError(t, cache.Set(context.Background(), key.PostDetail(11), "cached"))
	require.NoError(t, cache.Set(context.Background(), key.CommentList(11, 10, 0), "cached"))
	require.NoError(t, cache.Set(context.Background(), key.ReactionList(string(entity.ReactionTargetPost), 11), "cached"))
	require.NoError(t, cache.Set(context.Background(), key.TagPostList("go", 10, 0), "cached"))

	require.NoError(t, h.Handle(context.Background(), NewBoardChanged("visibility", 2)))

	for _, cacheKey := range []string{
		key.PostDetail(11),
		key.CommentList(11, 10, 0),
		key.ReactionList(string(entity.ReactionTargetPost), 11),
		key.TagPostList("go", 10, 0),
	} {
		_, ok, err := cache.Get(context.Background(), cacheKey)
		require.NoError(t, err)
		assert.False(t, ok)
	}
}

func TestCacheInvalidationHandler_PostChangedDelete(t *testing.T) {
	cache := testutil.NewSpyCache()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h := NewCacheInvalidationHandler(cache, logger)

	require.NoError(t, cache.Set(context.Background(), key.PostDetail(10), "cached"))
	require.NoError(t, cache.Set(context.Background(), key.PostList(2, 10, 0), "cached"))
	require.NoError(t, cache.Set(context.Background(), key.TagPostList("go", 10, 0), "cached"))
	require.NoError(t, cache.Set(context.Background(), key.CommentList(10, 10, 0), "cached"))
	require.NoError(t, cache.Set(context.Background(), key.ReactionList(string(entity.ReactionTargetPost), 10), "cached"))
	require.NoError(t, cache.Set(context.Background(), key.ReactionList(string(entity.ReactionTargetComment), 100), "cached"))

	e := NewPostChanged("deleted", 10, 2, []string{"go"}, []int64{100})
	require.NoError(t, h.Handle(context.Background(), e))

	for _, cacheKey := range []string{
		key.PostDetail(10),
		key.PostList(2, 10, 0),
		key.TagPostList("go", 10, 0),
		key.CommentList(10, 10, 0),
		key.ReactionList(string(entity.ReactionTargetPost), 10),
		key.ReactionList(string(entity.ReactionTargetComment), 100),
	} {
		_, ok, err := cache.Get(context.Background(), cacheKey)
		require.NoError(t, err)
		assert.False(t, ok)
	}
}

func TestCacheInvalidationHandler_CommentReactionAttachmentChanged(t *testing.T) {
	cache := testutil.NewSpyCache()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h := NewCacheInvalidationHandler(cache, logger)

	require.NoError(t, cache.Set(context.Background(), key.CommentList(11, 10, 0), "cached"))
	require.NoError(t, cache.Set(context.Background(), key.PostDetail(11), "cached"))
	require.NoError(t, cache.Set(context.Background(), key.ReactionList(string(entity.ReactionTargetComment), 90), "cached"))
	require.NoError(t, h.Handle(context.Background(), NewCommentChanged("deleted", 90, 11)))

	require.NoError(t, cache.Set(context.Background(), key.ReactionList(string(entity.ReactionTargetPost), 11), "cached"))
	require.NoError(t, cache.Set(context.Background(), key.PostDetail(11), "cached"))
	require.NoError(t, h.Handle(context.Background(), NewReactionChanged("set", entity.ReactionTargetPost, 11, 11)))

	require.NoError(t, cache.Set(context.Background(), key.PostDetail(11), "cached"))
	require.NoError(t, h.Handle(context.Background(), NewAttachmentChanged("deleted", 5, 11)))

	for _, cacheKey := range []string{
		key.CommentList(11, 10, 0),
		key.PostDetail(11),
		key.ReactionList(string(entity.ReactionTargetComment), 90),
		key.ReactionList(string(entity.ReactionTargetPost), 11),
	} {
		_, ok, err := cache.Get(context.Background(), cacheKey)
		require.NoError(t, err)
		assert.False(t, ok)
	}
}

type recordingCache struct {
	deleteCtx         context.Context
	deleteByPrefixCtx context.Context
}

func (c *recordingCache) Get(context.Context, string) (interface{}, bool, error) {
	return nil, false, nil
}
func (c *recordingCache) Set(context.Context, string, interface{}) error { return nil }
func (c *recordingCache) SetWithTTL(context.Context, string, interface{}, int) error {
	return nil
}
func (c *recordingCache) Delete(ctx context.Context, key string) error {
	c.deleteCtx = ctx
	_ = key
	return nil
}
func (c *recordingCache) DeleteByPrefix(ctx context.Context, prefix string) (int, error) {
	c.deleteByPrefixCtx = ctx
	_ = prefix
	return 0, nil
}
func (c *recordingCache) ExistsByPrefix(context.Context, string) (bool, error) {
	return false, nil
}
func (c *recordingCache) GetOrSetWithTTL(ctx context.Context, key string, ttlSeconds int, loader func(context.Context) (interface{}, error)) (interface{}, error) {
	return loader(ctx)
}

var _ port.Cache = (*recordingCache)(nil)

func TestCacheInvalidationHandler_UsesProvidedContext(t *testing.T) {
	cache := &recordingCache{}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h := NewCacheInvalidationHandler(cache, logger)

	ctx := context.WithValue(context.Background(), struct{ k string }{k: "rid"}, "rid-1")
	require.NoError(t, h.Handle(ctx, NewPostChanged("updated", 10, 2, []string{"go"}, nil)))
	assert.Same(t, ctx, cache.deleteCtx)
	assert.Same(t, ctx, cache.deleteByPrefixCtx)
}
