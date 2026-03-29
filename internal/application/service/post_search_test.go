package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	postsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/post"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostService_SearchPosts_InvalidQuery(t *testing.T) {
	repositories := newTestRepositories()
	svc := newTestPostService(t, repositories, newTestCache())

	_, err := svc.SearchPosts(context.Background(), "   ", "", "", 10, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))
}

func TestPostService_SearchPosts_TopPropagatesDefaultAndExplicitWindows(t *testing.T) {
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
			rebuildSearchIndex(t, repositories)

			list, err := postSvc.SearchPosts(context.Background(), "top", "top", tc.window, 10, "")
			require.NoError(t, err)
			require.Len(t, list.Posts, 1)
			assert.Equal(t, postUUID, list.Posts[0].UUID)
			require.Equal(t, 1, recordingRanking.listFeedCalls)
			assert.Equal(t, port.PostFeedSortTop, recordingRanking.lastSort)
			assert.Equal(t, tc.wantWindow, recordingRanking.lastWindow)
		})
	}
}

func TestPostService_SearchPosts_IndexesCreatedPostViaOutboxRelay(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := newTestPostService(t, repositories, newTestCache())

	postUUID, err := svc.CreatePost(context.Background(), "go search", "body", []string{"backend"}, nil, userID, mustBoardUUID(t, repositories.board, boardID))
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		list, err := svc.SearchPosts(context.Background(), "go search", "", "", 10, "")
		if err != nil {
			return false
		}
		return len(list.Posts) == 1 && list.Posts[0].UUID == postUUID
	}, time.Second, 10*time.Millisecond)
}

func TestPostService_SearchPosts_MatchesTitleContentAndTag(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	titlePostID := seedPost(repositories.post, userID, boardID, "Go search", "body")
	contentPostID := seedPost(repositories.post, userID, boardID, "title", "search token in body")
	tagPostID := seedPost(repositories.post, userID, boardID, "other", "body")
	require.NoError(t, repositories.unitOfWork.WithinTransaction(context.Background(), func(tx port.TxScope) error {
		return postsvc.NewTagCoordinator(repositories.tag, repositories.postTag).UpsertPostTags(tx, tagPostID, []string{"search"})
	}))
	rebuildSearchIndex(t, repositories)

	svc := newTestPostService(t, repositories, newTestCache())

	list, err := svc.SearchPosts(context.Background(), "search", "", "", 10, "")
	require.NoError(t, err)
	require.Len(t, list.Posts, 3)
	assert.Equal(t, mustPostUUID(t, repositories.post, titlePostID), list.Posts[0].UUID)
	assert.Equal(t, mustPostUUID(t, repositories.post, tagPostID), list.Posts[1].UUID)
	assert.Equal(t, mustPostUUID(t, repositories.post, contentPostID), list.Posts[2].UUID)
}

func TestPostService_SearchPosts_AllTermsMatchAndHiddenBoardsExcluded(t *testing.T) {
	repositories := newTestRepositories()
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	userID := seedUser(repositories.user, "alice", "pw", "user")
	visibleBoardID := seedBoard(repositories.board, "free", "desc")
	hiddenBoardID := seedBoard(repositories.board, "hidden", "desc")
	matchingVisibleID := seedPost(repositories.post, userID, visibleBoardID, "go search", "two terms")
	seedPost(repositories.post, userID, visibleBoardID, "go only", "one term")
	hiddenMatchID := seedPost(repositories.post, userID, hiddenBoardID, "go search", "hidden match")
	draftID := seedDraftPost(repositories.post, userID, visibleBoardID, "go search", "draft match")
	boardSvc := newTestBoardService(t, repositories, newTestCache())
	require.NoError(t, boardSvc.SetBoardVisibility(context.Background(), mustBoardUUID(t, repositories.board, hiddenBoardID), adminID, true))
	rebuildSearchIndex(t, repositories)
	svc := newTestPostService(t, repositories, newTestCache())

	list, err := svc.SearchPosts(context.Background(), "go search", "", "", 10, "")
	require.NoError(t, err)
	require.Len(t, list.Posts, 1)
	assert.Equal(t, mustPostUUID(t, repositories.post, matchingVisibleID), list.Posts[0].UUID)
	assert.NotEqual(t, mustPostUUID(t, repositories.post, hiddenMatchID), list.Posts[0].UUID)
	assert.NotEqual(t, mustPostUUID(t, repositories.post, draftID), list.Posts[0].UUID)
}

func TestPostQueryHandler_SearchPosts_UsesCompositeCursorPagination(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	firstID := seedPost(repositories.post, userID, boardID, "alpha beta", "alpha beta")
	secondID := seedPost(repositories.post, userID, boardID, "alpha beta", "alpha beta")
	thirdID := seedPost(repositories.post, userID, boardID, "alpha beta", "alpha beta")
	rebuildSearchIndex(t, repositories)
	query := postsvc.NewQueryHandler(
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
		policy.NewRoleAuthorizationPolicy(),
		newTestCache(),
		newTestCachePolicy(),
	)

	page1, err := query.SearchPosts(context.Background(), "alpha beta", "", "", 2, "")
	require.NoError(t, err)
	require.Len(t, page1.Posts, 2)
	require.True(t, page1.HasMore)
	require.NotNil(t, page1.NextCursor)
	assert.Equal(t, mustPostUUID(t, repositories.post, thirdID), page1.Posts[0].UUID)
	assert.Equal(t, mustPostUUID(t, repositories.post, secondID), page1.Posts[1].UUID)

	page2, err := query.SearchPosts(context.Background(), "alpha beta", "", "", 2, *page1.NextCursor)
	require.NoError(t, err)
	require.Len(t, page2.Posts, 1)
	assert.Equal(t, mustPostUUID(t, repositories.post, firstID), page2.Posts[0].UUID)
	assert.False(t, page2.HasMore)
	assert.Nil(t, page2.NextCursor)
}

func TestPostQueryHandler_SearchPosts_RankedCursorPaginationUsesFeedCursor(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	firstID := seedPost(repositories.post, userID, boardID, "alpha beta", "alpha beta")
	secondID := seedPost(repositories.post, userID, boardID, "alpha beta", "alpha beta")
	thirdID := seedPost(repositories.post, userID, boardID, "alpha beta", "alpha beta")

	base := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
	require.NoError(t, repositories.postRanking.UpsertPostSnapshot(context.Background(), firstID, boardID, ptrTime(base.Add(-2*time.Hour)), entity.PostStatusPublished))
	require.NoError(t, repositories.postRanking.UpsertPostSnapshot(context.Background(), secondID, boardID, ptrTime(base.Add(-1*time.Hour)), entity.PostStatusPublished))
	require.NoError(t, repositories.postRanking.UpsertPostSnapshot(context.Background(), thirdID, boardID, ptrTime(base), entity.PostStatusPublished))
	rebuildSearchIndex(t, repositories)

	query := postsvc.NewQueryHandler(
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
		policy.NewRoleAuthorizationPolicy(),
		newTestCache(),
		newTestCachePolicy(),
	)

	page1, err := query.SearchPosts(context.Background(), "alpha beta", "latest", "", 2, "")
	require.NoError(t, err)
	require.Len(t, page1.Posts, 2)
	require.True(t, page1.HasMore)
	require.NotNil(t, page1.NextCursor)
	assert.Equal(t, mustPostUUID(t, repositories.post, thirdID), page1.Posts[0].UUID)
	assert.Equal(t, mustPostUUID(t, repositories.post, secondID), page1.Posts[1].UUID)

	page2, err := query.SearchPosts(context.Background(), "alpha beta", "latest", "", 2, *page1.NextCursor)
	require.NoError(t, err)
	require.Len(t, page2.Posts, 1)
	assert.Equal(t, mustPostUUID(t, repositories.post, firstID), page2.Posts[0].UUID)
	assert.False(t, page2.HasMore)
	assert.Nil(t, page2.NextCursor)
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func TestPostQueryHandler_SearchPosts_InvalidCursor(t *testing.T) {
	repositories := newTestRepositories()
	query := postsvc.NewQueryHandler(
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
		policy.NewRoleAuthorizationPolicy(),
		newTestCache(),
		newTestCachePolicy(),
	)

	_, err := query.SearchPosts(context.Background(), "go", "", "", 10, "bad-cursor")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))
}

func TestPostQueryHandler_SearchPosts_WithoutRepositoryFails(t *testing.T) {
	repositories := newTestRepositories()
	query := postsvc.NewQueryHandler(
		repositories.user,
		repositories.board,
		repositories.post,
		nil,
		repositories.postRanking,
		repositories.tag,
		repositories.postTag,
		repositories.attachment,
		repositories.comment,
		repositories.reaction,
		policy.NewRoleAuthorizationPolicy(),
		newTestCache(),
		newTestCachePolicy(),
	)

	_, err := query.SearchPosts(context.Background(), "go", "", "", 10, "")
	require.Error(t, err)
}
