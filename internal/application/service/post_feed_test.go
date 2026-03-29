package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingPostRepository struct {
	port.PostRepository
	singleLookupCount int
	batchLookupCount  int
}

func (r *countingPostRepository) SelectPostByIDIncludingUnpublished(ctx context.Context, id int64) (*entity.Post, error) {
	r.singleLookupCount++
	return r.PostRepository.SelectPostByIDIncludingUnpublished(ctx, id)
}

func (r *countingPostRepository) SelectPostsByIDsIncludingUnpublished(ctx context.Context, ids []int64) (map[int64]*entity.Post, error) {
	r.batchLookupCount++
	return r.PostRepository.SelectPostsByIDsIncludingUnpublished(ctx, ids)
}

type recordingPostRankingRepository struct {
	port.PostRankingRepository
	listFeedCalls int
	lastSort      port.PostFeedSort
	lastWindow    port.PostRankingWindow
	lastLimit     int
	lastCursor    *port.PostFeedCursor
}

func (r *recordingPostRankingRepository) ListFeed(ctx context.Context, sortBy port.PostFeedSort, window port.PostRankingWindow, limit int, cursor *port.PostFeedCursor) ([]port.PostFeedResult, error) {
	r.listFeedCalls++
	r.lastSort = sortBy
	r.lastWindow = window
	r.lastLimit = limit
	r.lastCursor = cursor
	return r.PostRankingRepository.ListFeed(ctx, sortBy, window, limit, cursor)
}

func TestPostService_GetFeed_DefaultsToHotAndReflectsRankingEvents(t *testing.T) {
	repositories := newTestRepositories()
	cache := newTestCache()
	actionDispatcher := newTestActionDispatcher(t, repositories, cache)
	postSvc := NewPostServiceWithActionDispatcher(
		repositories.user,
		repositories.board,
		repositories.post,
		repositories.postSearch,
		repositories.postRanking,
		repositories.tag,
		repositories.postTag,
		repositories.attachment,
		repositories.comment,
		repositories.reaction,
		repositories.unitOfWork,
		cache,
		actionDispatcher,
		newTestCachePolicy(),
		newTestAuthorizationPolicy(),
	)
	commentSvc := NewCommentServiceWithActionDispatcher(
		repositories.user,
		repositories.board,
		repositories.post,
		repositories.comment,
		repositories.reaction,
		repositories.unitOfWork,
		cache,
		actionDispatcher,
		newTestCachePolicy(),
		newTestAuthorizationPolicy(),
	)
	reactionSvc := NewReactionServiceWithActionDispatcher(
		repositories.user,
		repositories.board,
		repositories.post,
		repositories.comment,
		repositories.reaction,
		repositories.unitOfWork,
		cache,
		actionDispatcher,
		newTestCachePolicy(),
	)

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	boardUUID := mustBoardUUID(t, repositories.board, boardID)

	firstUUID, err := postSvc.CreatePost(context.Background(), "older", "body", nil, nil, userID, boardUUID)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	secondUUID, err := postSvc.CreatePost(context.Background(), "newer", "body", nil, nil, userID, boardUUID)
	require.NoError(t, err)

	_, err = reactionSvc.SetReaction(context.Background(), userID, firstUUID, "post", "like")
	require.NoError(t, err)
	_, err = commentSvc.CreateComment(context.Background(), "boost", nil, userID, firstUUID, nil)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		list, err := postSvc.GetFeed(context.Background(), "", "", 10, "")
		if err != nil || len(list.Posts) < 2 {
			return false
		}
		return list.Posts[0].UUID == firstUUID && list.Posts[1].UUID == secondUUID
	}, time.Second, 10*time.Millisecond)
}

func TestPostService_GetFeed_BatchesRankedPostLookup(t *testing.T) {
	repositories := newTestRepositories()
	countingPostRepo := &countingPostRepository{PostRepository: repositories.post}
	repositories.post = countingPostRepo

	cache := newTestCache()
	actionDispatcher := newTestActionDispatcher(t, repositories, cache)
	postSvc := NewPostServiceWithActionDispatcher(
		repositories.user,
		repositories.board,
		repositories.post,
		repositories.postSearch,
		repositories.postRanking,
		repositories.tag,
		repositories.postTag,
		repositories.attachment,
		repositories.comment,
		repositories.reaction,
		repositories.unitOfWork,
		cache,
		actionDispatcher,
		newTestCachePolicy(),
		newTestAuthorizationPolicy(),
	)

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	boardUUID := mustBoardUUID(t, repositories.board, boardID)

	_, err := postSvc.CreatePost(context.Background(), "older", "body", nil, nil, userID, boardUUID)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	_, err = postSvc.CreatePost(context.Background(), "newer", "body", nil, nil, userID, boardUUID)
	require.NoError(t, err)
	_, err = postSvc.CreatePost(context.Background(), "newest", "body", nil, nil, userID, boardUUID)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		results, err := repositories.postRanking.ListFeed(context.Background(), "hot", "", 10, nil)
		if err != nil {
			return false
		}
		return len(results) == 3
	}, time.Second, 10*time.Millisecond)

	list, err := postSvc.GetFeed(context.Background(), "", "", 10, "")
	require.NoError(t, err)
	require.Len(t, list.Posts, 3)
	assert.Equal(t, 1, countingPostRepo.batchLookupCount)
	assert.Zero(t, countingPostRepo.singleLookupCount)
}

func TestPostService_GetFeed_HidesDraftsAndHiddenBoards(t *testing.T) {
	repositories := newTestRepositories()
	cache := newTestCache()
	actionDispatcher := newTestActionDispatcher(t, repositories, cache)
	postSvc := NewPostServiceWithActionDispatcher(
		repositories.user,
		repositories.board,
		repositories.post,
		repositories.postSearch,
		repositories.postRanking,
		repositories.tag,
		repositories.postTag,
		repositories.attachment,
		repositories.comment,
		repositories.reaction,
		repositories.unitOfWork,
		cache,
		actionDispatcher,
		newTestCachePolicy(),
		newTestAuthorizationPolicy(),
	)
	boardSvc := NewBoardServiceWithActionDispatcher(
		repositories.user,
		repositories.board,
		repositories.post,
		repositories.unitOfWork,
		cache,
		actionDispatcher,
		newTestCachePolicy(),
		policy.NewRoleAuthorizationPolicy(),
	)

	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	userID := seedUser(repositories.user, "alice", "pw", "user")
	visibleBoardID := seedBoard(repositories.board, "free", "desc")
	hiddenBoardID := seedBoard(repositories.board, "hidden", "desc")

	visibleUUID, err := postSvc.CreatePost(context.Background(), "visible", "body", nil, nil, userID, mustBoardUUID(t, repositories.board, visibleBoardID))
	require.NoError(t, err)
	_, err = postSvc.CreateDraftPost(context.Background(), "draft", "body", nil, nil, userID, mustBoardUUID(t, repositories.board, visibleBoardID))
	require.NoError(t, err)
	_, err = postSvc.CreatePost(context.Background(), "hidden", "body", nil, nil, userID, mustBoardUUID(t, repositories.board, hiddenBoardID))
	require.NoError(t, err)
	require.NoError(t, boardSvc.SetBoardVisibility(context.Background(), mustBoardUUID(t, repositories.board, hiddenBoardID), adminID, true))

	require.Eventually(t, func() bool {
		list, err := postSvc.GetFeed(context.Background(), "latest", "", 10, "")
		if err != nil || len(list.Posts) != 1 {
			return false
		}
		return list.Posts[0].UUID == visibleUUID
	}, time.Second, 10*time.Millisecond)
}

func TestPostService_GetFeed_PublishedDraftAppearsWithoutFollowUpUpdate(t *testing.T) {
	repositories := newTestRepositories()
	cache := newTestCache()
	actionDispatcher := newTestActionDispatcher(t, repositories, cache)
	postSvc := NewPostServiceWithActionDispatcher(
		repositories.user,
		repositories.board,
		repositories.post,
		repositories.postSearch,
		repositories.postRanking,
		repositories.tag,
		repositories.postTag,
		repositories.attachment,
		repositories.comment,
		repositories.reaction,
		repositories.unitOfWork,
		cache,
		actionDispatcher,
		newTestCachePolicy(),
		newTestAuthorizationPolicy(),
	)

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	boardUUID := mustBoardUUID(t, repositories.board, boardID)

	postUUID, err := postSvc.CreateDraftPost(context.Background(), "draft", "body", nil, nil, userID, boardUUID)
	require.NoError(t, err)
	require.NoError(t, postSvc.PublishPost(context.Background(), postUUID, userID))

	require.Eventually(t, func() bool {
		list, err := postSvc.GetFeed(context.Background(), "latest", "", 10, "")
		if err != nil || len(list.Posts) != 1 {
			return false
		}
		return list.Posts[0].UUID == postUUID
	}, time.Second, 10*time.Millisecond)
}

func TestPostService_GetFeed_RejectsInvalidSortAndCursor(t *testing.T) {
	repositories := newTestRepositories()
	svc := newTestPostService(t, repositories, newTestCache())

	_, err := svc.GetFeed(context.Background(), "weird", "", 10, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))

	_, err = svc.GetFeed(context.Background(), "hot", "", 10, "bad-cursor")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))
}

func TestPostService_GetFeed_TopPropagatesDefaultAndExplicitWindows(t *testing.T) {
	cases := []struct {
		name       string
		window     string
		wantWindow port.PostRankingWindow
	}{
		{name: "default", window: "", wantWindow: port.PostRankingWindow7d},
		{name: "24h", window: "24h", wantWindow: port.PostRankingWindow24h},
		{name: "7d", window: "7d", wantWindow: port.PostRankingWindow7d},
		{name: "30d", window: "30d", wantWindow: port.PostRankingWindow30d},
		{name: "all", window: "all", wantWindow: port.PostRankingWindowAll},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repositories := newTestRepositories()
			recordingRanking := &recordingPostRankingRepository{PostRankingRepository: repositories.postRanking}
			repositories.postRanking = recordingRanking
			cache := newTestCache()
			actionDispatcher := newTestActionDispatcher(t, repositories, cache)
			postSvc := NewPostServiceWithActionDispatcher(
				repositories.user,
				repositories.board,
				repositories.post,
				repositories.postSearch,
				repositories.postRanking,
				repositories.tag,
				repositories.postTag,
				repositories.attachment,
				repositories.comment,
				repositories.reaction,
				repositories.unitOfWork,
				cache,
				actionDispatcher,
				newTestCachePolicy(),
				newTestAuthorizationPolicy(),
			)
			userID := seedUser(repositories.user, "alice", "pw", "user")
			boardID := seedBoard(repositories.board, "free", "desc")
			boardUUID := mustBoardUUID(t, repositories.board, boardID)
			postUUID, err := postSvc.CreatePost(context.Background(), "top title", "top body", nil, nil, userID, boardUUID)
			require.NoError(t, err)
			createdPost, err := repositories.post.SelectPostByUUID(context.Background(), postUUID)
			require.NoError(t, err)
			require.NotNil(t, createdPost)
			require.NoError(t, repositories.postRanking.UpsertPostSnapshot(context.Background(), createdPost.ID, createdPost.BoardID, createdPost.PublishedAt, createdPost.Status))

			list, err := postSvc.GetFeed(context.Background(), "top", tc.window, 10, "")
			require.NoError(t, err)
			require.Len(t, list.Posts, 1)
			assert.Equal(t, postUUID, list.Posts[0].UUID)
			require.Equal(t, 1, recordingRanking.listFeedCalls)
			assert.Equal(t, port.PostFeedSortTop, recordingRanking.lastSort)
			assert.Equal(t, tc.wantWindow, recordingRanking.lastWindow)
		})
	}
}
