package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	postsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/post"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
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
		newTestCache(),
		newTestCachePolicy(),
	)

	_, err := query.SearchPosts(context.Background(), "go", "", "", 10, "")
	require.Error(t, err)
}
