package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
