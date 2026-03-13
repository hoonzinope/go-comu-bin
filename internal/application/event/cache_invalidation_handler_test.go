package event

import (
	"io"
	"log/slog"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/testutil"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheInvalidationHandler_BoardChanged(t *testing.T) {
	cache := testutil.NewSpyCache()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h := NewCacheInvalidationHandler(cache, logger)

	require.NoError(t, cache.Set(key.BoardList(10, 0), "cached"))
	require.NoError(t, h.Handle(NewBoardChanged("updated", 1)))

	_, ok, err := cache.Get(key.BoardList(10, 0))
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCacheInvalidationHandler_PostChangedDelete(t *testing.T) {
	cache := testutil.NewSpyCache()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h := NewCacheInvalidationHandler(cache, logger)

	require.NoError(t, cache.Set(key.PostDetail(10), "cached"))
	require.NoError(t, cache.Set(key.PostList(2, 10, 0), "cached"))
	require.NoError(t, cache.Set(key.TagPostList("go", 10, 0), "cached"))
	require.NoError(t, cache.Set(key.CommentList(10, 10, 0), "cached"))
	require.NoError(t, cache.Set(key.ReactionList(string(entity.ReactionTargetPost), 10), "cached"))
	require.NoError(t, cache.Set(key.ReactionList(string(entity.ReactionTargetComment), 100), "cached"))

	e := NewPostChanged("deleted", 10, 2, []string{"go"}, []int64{100})
	require.NoError(t, h.Handle(e))

	for _, cacheKey := range []string{
		key.PostDetail(10),
		key.PostList(2, 10, 0),
		key.TagPostList("go", 10, 0),
		key.CommentList(10, 10, 0),
		key.ReactionList(string(entity.ReactionTargetPost), 10),
		key.ReactionList(string(entity.ReactionTargetComment), 100),
	} {
		_, ok, err := cache.Get(cacheKey)
		require.NoError(t, err)
		assert.False(t, ok)
	}
}

func TestCacheInvalidationHandler_CommentReactionAttachmentChanged(t *testing.T) {
	cache := testutil.NewSpyCache()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h := NewCacheInvalidationHandler(cache, logger)

	require.NoError(t, cache.Set(key.CommentList(11, 10, 0), "cached"))
	require.NoError(t, cache.Set(key.PostDetail(11), "cached"))
	require.NoError(t, cache.Set(key.ReactionList(string(entity.ReactionTargetComment), 90), "cached"))
	require.NoError(t, h.Handle(NewCommentChanged("deleted", 90, 11)))

	require.NoError(t, cache.Set(key.ReactionList(string(entity.ReactionTargetPost), 11), "cached"))
	require.NoError(t, cache.Set(key.PostDetail(11), "cached"))
	require.NoError(t, h.Handle(NewReactionChanged("set", entity.ReactionTargetPost, 11, 11)))

	require.NoError(t, cache.Set(key.PostDetail(11), "cached"))
	require.NoError(t, h.Handle(NewAttachmentChanged("deleted", 5, 11)))

	for _, cacheKey := range []string{
		key.CommentList(11, 10, 0),
		key.PostDetail(11),
		key.ReactionList(string(entity.ReactionTargetComment), 90),
		key.ReactionList(string(entity.ReactionTargetPost), 11),
	} {
		_, ok, err := cache.Get(cacheKey)
		require.NoError(t, err)
		assert.False(t, ok)
	}
}
