package inmemory

import (
	"context"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostRankingRepository_ListFeedLatestWithCursor(t *testing.T) {
	repo := NewPostRankingRepository()
	ctx := context.Background()
	now := time.Now()
	first := now.Add(-3 * time.Hour)
	second := now.Add(-2 * time.Hour)
	third := now.Add(-1 * time.Hour)

	require.NoError(t, repo.UpsertPostSnapshot(ctx, 1, 1, &first, entity.PostStatusPublished))
	require.NoError(t, repo.UpsertPostSnapshot(ctx, 2, 1, &second, entity.PostStatusPublished))
	require.NoError(t, repo.UpsertPostSnapshot(ctx, 3, 1, &third, entity.PostStatusPublished))

	page1, err := repo.ListFeed(ctx, port.PostFeedSortLatest, "", 2, nil)
	require.NoError(t, err)
	require.Len(t, page1, 2)
	assert.Equal(t, int64(3), page1[0].PostID)
	assert.Equal(t, int64(2), page1[1].PostID)

	cursor := &port.PostFeedCursor{
		Sort:                port.PostFeedSortLatest,
		Window:              "",
		PublishedAtUnixNano: page1[1].PublishedAt.UnixNano(),
		PostID:              page1[1].PostID,
	}
	page2, err := repo.ListFeed(ctx, port.PostFeedSortLatest, "", 2, cursor)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	assert.Equal(t, int64(1), page2[0].PostID)
}

func TestPostRankingRepository_ListFeedBestPrunesExpiredActivity(t *testing.T) {
	repo := NewPostRankingRepository()
	ctx := context.Background()
	now := time.Now()
	publishedAt := now.Add(-24 * time.Hour)
	otherPublishedAt := now.Add(-48 * time.Hour)

	require.NoError(t, repo.UpsertPostSnapshot(ctx, 1, 1, &publishedAt, entity.PostStatusPublished))
	require.NoError(t, repo.UpsertPostSnapshot(ctx, 2, 1, &otherPublishedAt, entity.PostStatusPublished))

	require.NoError(t, repo.ApplyReactionDelta(ctx, 1, 1, 10, entity.ReactionTargetPost, entity.ReactionTypeLike, "set", now.Add(-time.Hour)))
	require.NoError(t, repo.ApplyCommentDelta(ctx, 1, 20, now.Add(-30*time.Minute), 2))
	require.NoError(t, repo.ApplyCommentDelta(ctx, 2, 30, now.Add(-8*24*time.Hour), 2))

	results, err := repo.ListFeed(ctx, port.PostFeedSortBest, "", 10, nil)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, int64(1), results[0].PostID)
	assert.Equal(t, float64(3), results[0].Score)
	assert.Equal(t, int64(2), results[1].PostID)
	assert.Equal(t, float64(0), results[1].Score)
}

func TestPostRankingRepository_ApplyReactionDelta_DeduplicatesSetAndUnset(t *testing.T) {
	repo := NewPostRankingRepository()
	ctx := context.Background()
	now := time.Now()
	publishedAt := now.Add(-time.Hour)

	require.NoError(t, repo.UpsertPostSnapshot(ctx, 1, 1, &publishedAt, entity.PostStatusPublished))

	require.NoError(t, repo.ApplyReactionDelta(ctx, 1, 1, 10, entity.ReactionTargetPost, entity.ReactionTypeLike, "set", now.Add(-10*time.Minute)))
	require.NoError(t, repo.ApplyReactionDelta(ctx, 1, 1, 10, entity.ReactionTargetPost, entity.ReactionTypeLike, "set", now.Add(-9*time.Minute)))
	results, err := repo.ListFeed(ctx, port.PostFeedSortBest, "", 10, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, float64(1), results[0].Score)

	require.NoError(t, repo.ApplyReactionDelta(ctx, 1, 1, 10, entity.ReactionTargetPost, entity.ReactionTypeDislike, "set", now.Add(-8*time.Minute)))
	results, err = repo.ListFeed(ctx, port.PostFeedSortBest, "", 10, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, float64(-1), results[0].Score)

	require.NoError(t, repo.ApplyReactionDelta(ctx, 1, 1, 10, entity.ReactionTargetPost, entity.ReactionTypeDislike, "unset", now.Add(-7*time.Minute)))
	results, err = repo.ListFeed(ctx, port.PostFeedSortBest, "", 10, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, float64(0), results[0].Score)
}

func TestPostRankingRepository_ListFeedTopSupportsWindows(t *testing.T) {
	repo := NewPostRankingRepository()
	ctx := context.Background()
	now := time.Now()
	firstPublishedAt := now.Add(-10 * 24 * time.Hour)
	secondPublishedAt := now.Add(-2 * time.Hour)

	require.NoError(t, repo.UpsertPostSnapshot(ctx, 1, 1, &firstPublishedAt, entity.PostStatusPublished))
	require.NoError(t, repo.UpsertPostSnapshot(ctx, 2, 1, &secondPublishedAt, entity.PostStatusPublished))

	require.NoError(t, repo.ApplyCommentDelta(ctx, 1, 11, now.Add(-8*24*time.Hour), 2))
	require.NoError(t, repo.ApplyCommentDelta(ctx, 1, 12, now.Add(-2*time.Hour), 2))
	require.NoError(t, repo.ApplyReactionDelta(ctx, 2, 2, 22, entity.ReactionTargetPost, entity.ReactionTypeLike, "set", now.Add(-time.Hour)))

	top24h, err := repo.ListFeed(ctx, port.PostFeedSortTop, port.PostRankingWindow24h, 10, nil)
	require.NoError(t, err)
	require.Len(t, top24h, 2)
	assert.Equal(t, int64(1), top24h[0].PostID)
	assert.Equal(t, float64(2), top24h[0].Score)
	assert.Equal(t, int64(2), top24h[1].PostID)
	assert.Equal(t, float64(1), top24h[1].Score)

	top30d, err := repo.ListFeed(ctx, port.PostFeedSortTop, port.PostRankingWindow30d, 10, nil)
	require.NoError(t, err)
	require.Len(t, top30d, 2)
	assert.Equal(t, int64(1), top30d[0].PostID)
	assert.Equal(t, float64(4), top30d[0].Score)

	topAll, err := repo.ListFeed(ctx, port.PostFeedSortTop, port.PostRankingWindowAll, 10, nil)
	require.NoError(t, err)
	require.Len(t, topAll, 2)
	assert.Equal(t, float64(4), topAll[0].Score)
}
