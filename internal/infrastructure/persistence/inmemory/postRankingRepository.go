package inmemory

import (
	"context"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

const redditEpoch = 1134028003
const hotDecayDivisor = 45000.0
const (
	bestWindow      = 7 * 24 * time.Hour
	top24hWindow    = 24 * time.Hour
	top7dWindow     = 7 * 24 * time.Hour
	top30dWindow    = 30 * 24 * time.Hour
	maxActivitySpan = top30dWindow
)

type PostRankingRepository struct {
	mu             sync.RWMutex
	posts          map[int64]*rankingPostSnapshot
	reactionStates map[string]entity.ReactionType
	commentStates  map[int64]int
}

type rankingPostSnapshot struct {
	PostID             int64
	BoardID            int64
	PublishedAt        time.Time
	Status             entity.PostStatus
	TotalReactionScore int
	TotalCommentScore  int
	Activity           []rankingActivity
}

type rankingActivity struct {
	OccurredAt time.Time
	Weight     int
}

var _ port.PostRankingRepository = (*PostRankingRepository)(nil)

func NewPostRankingRepository() *PostRankingRepository {
	return &PostRankingRepository{
		posts:          make(map[int64]*rankingPostSnapshot),
		reactionStates: make(map[string]entity.ReactionType),
		commentStates:  make(map[int64]int),
	}
}

func (r *PostRankingRepository) UpsertPostSnapshot(ctx context.Context, postID, boardID int64, publishedAt *time.Time, status entity.PostStatus) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.posts[postID]
	if !ok {
		item = &rankingPostSnapshot{PostID: postID}
		r.posts[postID] = item
	}
	item.BoardID = boardID
	item.Status = status
	if publishedAt != nil {
		item.PublishedAt = *publishedAt
	}
	return nil
}

func (r *PostRankingRepository) DeletePost(ctx context.Context, postID int64) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.posts, postID)
	return nil
}

func (r *PostRankingRepository) ApplyCommentDelta(ctx context.Context, postID, commentID int64, occurredAt time.Time, delta int) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	post, ok := r.posts[postID]
	if !ok || delta == 0 {
		return nil
	}
	current := r.commentStates[commentID]
	switch {
	case delta > 0:
		if current == delta {
			return nil
		}
		post.TotalCommentScore += delta - current
		r.commentStates[commentID] = delta
		post.Activity = append(post.Activity, rankingActivity{OccurredAt: occurredAt, Weight: delta - current})
	case delta < 0:
		if current == 0 {
			return nil
		}
		post.TotalCommentScore -= current
		delete(r.commentStates, commentID)
		post.Activity = append(post.Activity, rankingActivity{OccurredAt: occurredAt, Weight: -current})
	}
	return nil
}

func (r *PostRankingRepository) ApplyReactionDelta(ctx context.Context, postID, targetID, userID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType, operation string, occurredAt time.Time) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	post, ok := r.posts[postID]
	if !ok {
		return nil
	}
	key := reactionKey(targetType, targetID, userID)
	prev, exists := r.reactionStates[key]
	switch operation {
	case "set":
		if exists && prev == reactionType {
			return nil
		}
		delta := reactionWeight(reactionType)
		if exists {
			delta -= reactionWeight(prev)
		}
		if delta == 0 {
			r.reactionStates[key] = reactionType
			return nil
		}
		post.TotalReactionScore += delta
		r.reactionStates[key] = reactionType
		post.Activity = append(post.Activity, rankingActivity{OccurredAt: occurredAt, Weight: delta})
	case "unset":
		if !exists {
			return nil
		}
		delta := -reactionWeight(prev)
		post.TotalReactionScore += delta
		delete(r.reactionStates, key)
		post.Activity = append(post.Activity, rankingActivity{OccurredAt: occurredAt, Weight: delta})
	}
	return nil
}

func (r *PostRankingRepository) ListFeed(ctx context.Context, sortBy port.PostFeedSort, window port.PostRankingWindow, limit int, cursor *port.PostFeedCursor) ([]port.PostFeedResult, error) {
	_ = ctx
	if limit <= 0 {
		return []port.PostFeedResult{}, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	results := make([]port.PostFeedResult, 0, len(r.posts))
	for _, item := range r.posts {
		if item == nil || item.Status != entity.PostStatusPublished || item.PublishedAt.IsZero() {
			continue
		}
		item.Activity = pruneActivities(item.Activity, now.Add(-maxActivitySpan))
		score := rankingScore(sortBy, window, item, now)
		result := port.PostFeedResult{
			PostID:      item.PostID,
			BoardID:     item.BoardID,
			Score:       score,
			PublishedAt: item.PublishedAt,
		}
		if cursor != nil && !feedResultAfterCursor(result, *cursor) {
			continue
		}
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool {
		return compareFeedResults(results[i], results[j], sortBy)
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func pruneActivities(items []rankingActivity, cutoff time.Time) []rankingActivity {
	out := items[:0]
	for _, item := range items {
		if item.OccurredAt.Before(cutoff) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func rankingScore(sortBy port.PostFeedSort, window port.PostRankingWindow, item *rankingPostSnapshot, now time.Time) float64 {
	switch sortBy {
	case port.PostFeedSortLatest:
		return float64(item.PublishedAt.UnixNano())
	case port.PostFeedSortBest:
		return float64(activityScoreWithin(item.Activity, now.Add(-bestWindow)))
	case port.PostFeedSortTop:
		if window == port.PostRankingWindowAll {
			return float64(item.TotalReactionScore + item.TotalCommentScore)
		}
		return float64(activityScoreWithin(item.Activity, now.Add(-windowDuration(window))))
	default:
		base := float64(item.TotalReactionScore + item.TotalCommentScore)
		sign := 0.0
		if base > 0 {
			sign = 1
		} else if base < 0 {
			sign = -1
		}
		order := math.Log10(math.Max(math.Abs(base), 1))
		seconds := float64(item.PublishedAt.Unix() - redditEpoch)
		return sign*order + seconds/hotDecayDivisor
	}
}

func activityScoreWithin(items []rankingActivity, cutoff time.Time) int {
	score := 0
	for _, activity := range items {
		if activity.OccurredAt.Before(cutoff) {
			continue
		}
		score += activity.Weight
	}
	return score
}

func windowDuration(window port.PostRankingWindow) time.Duration {
	switch window {
	case port.PostRankingWindow24h:
		return top24hWindow
	case port.PostRankingWindow30d:
		return top30dWindow
	case port.PostRankingWindowAll:
		return 0
	default:
		return top7dWindow
	}
}

func compareFeedResults(left, right port.PostFeedResult, sortBy port.PostFeedSort) bool {
	switch sortBy {
	case port.PostFeedSortLatest:
		if left.PublishedAt.Equal(right.PublishedAt) {
			return left.PostID > right.PostID
		}
		return left.PublishedAt.After(right.PublishedAt)
	default:
		if left.Score == right.Score {
			if left.PublishedAt.Equal(right.PublishedAt) {
				return left.PostID > right.PostID
			}
			return left.PublishedAt.After(right.PublishedAt)
		}
		return left.Score > right.Score
	}
}

func feedResultAfterCursor(item port.PostFeedResult, cursor port.PostFeedCursor) bool {
	switch cursor.Sort {
	case port.PostFeedSortLatest:
		cursorTime := time.Unix(0, cursor.PublishedAtUnixNano)
		if item.PublishedAt.Before(cursorTime) {
			return true
		}
		if item.PublishedAt.After(cursorTime) {
			return false
		}
		return item.PostID < cursor.PostID
	default:
		if item.Score < cursor.Score {
			return true
		}
		if item.Score > cursor.Score {
			return false
		}
		cursorTime := time.Unix(0, cursor.PublishedAtUnixNano)
		if item.PublishedAt.Before(cursorTime) {
			return true
		}
		if item.PublishedAt.After(cursorTime) {
			return false
		}
		return item.PostID < cursor.PostID
	}
}

func reactionWeight(reactionType entity.ReactionType) int {
	switch reactionType {
	case entity.ReactionTypeLike:
		return 1
	case entity.ReactionTypeDislike:
		return -1
	default:
		return 0
	}
}

func reactionKey(targetType entity.ReactionTargetType, targetID, userID int64) string {
	return string(targetType) + ":" + strconv.FormatInt(targetID, 10) + ":" + strconv.FormatInt(userID, 10)
}
