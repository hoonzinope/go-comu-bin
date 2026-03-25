package port

import (
	"context"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type PostFeedSort string

const (
	PostFeedSortHot    PostFeedSort = "hot"
	PostFeedSortBest   PostFeedSort = "best"
	PostFeedSortLatest PostFeedSort = "latest"
	PostFeedSortTop    PostFeedSort = "top"
)

type PostRankingWindow string

const (
	PostRankingWindow24h PostRankingWindow = "24h"
	PostRankingWindow7d  PostRankingWindow = "7d"
	PostRankingWindow30d PostRankingWindow = "30d"
	PostRankingWindowAll PostRankingWindow = "all"
)

type PostFeedCursor struct {
	Sort                PostFeedSort
	Window              PostRankingWindow
	Score               float64
	PublishedAtUnixNano int64
	PostID              int64
}

type PostFeedResult struct {
	PostID      int64
	BoardID     int64
	Score       float64
	PublishedAt time.Time
}

type PostRankingRepository interface {
	UpsertPostSnapshot(ctx context.Context, postID, boardID int64, publishedAt *time.Time, status entity.PostStatus) error
	DeletePost(ctx context.Context, postID int64) error
	ApplyCommentDelta(ctx context.Context, postID, commentID int64, occurredAt time.Time, delta int) error
	ApplyReactionDelta(ctx context.Context, postID, targetID, userID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType, operation string, occurredAt time.Time) error
	ListFeed(ctx context.Context, sort PostFeedSort, window PostRankingWindow, limit int, cursor *PostFeedCursor) ([]PostFeedResult, error)
}
