package service

import (
	"errors"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/testutil"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommentService_UpdateComment_ForbiddenForNonOwnerNonAdmin(t *testing.T) {
	repositories := newTestRepositories()
	ownerID := seedUser(repositories.user, "owner", "pw", "user")
	otherID := seedUser(repositories.user, "other", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())
	commentID, err := svc.CreateComment("comment", ownerID, postID)
	require.NoError(t, err)

	err = svc.UpdateComment(commentID, otherID, "new-comment")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}

func TestCommentService_UpdateComment_AllowedForAdmin(t *testing.T) {
	repositories := newTestRepositories()
	ownerID := seedUser(repositories.user, "owner", "pw", "user")
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())
	commentID, err := svc.CreateComment("comment", ownerID, postID)
	require.NoError(t, err)

	require.NoError(t, svc.UpdateComment(commentID, adminID, "new-comment"))
}

func TestCommentService_CreateGetDelete_Success(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	commentID, err := svc.CreateComment("comment", userID, postID)
	require.NoError(t, err)
	assert.NotZero(t, commentID)

	list, err := svc.GetCommentsByPost(postID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, list.Comments, 1)

	require.NoError(t, svc.DeleteComment(commentID, userID))
}

func TestCommentService_CreateComment_InvalidInput(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err := svc.CreateComment(" ", userID, postID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}

func TestCommentService_GetCommentsByPost_HasMoreAndNextCursor(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	seedComment(repositories.comment, userID, postID, "c1")
	seedComment(repositories.comment, userID, postID, "c2")
	seedComment(repositories.comment, userID, postID, "c3")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	list, err := svc.GetCommentsByPost(postID, 2, 0)
	require.NoError(t, err)
	require.Len(t, list.Comments, 2)
	assert.True(t, list.HasMore)
	require.NotNil(t, list.NextLastID)
	assert.Equal(t, list.Comments[len(list.Comments)-1].ID, *list.NextLastID)
}

func TestCommentService_CreateComment_InvalidatesRelatedCaches(t *testing.T) {
	repositories := newTestRepositories()
	cache := testutil.NewSpyCache()
	commentSvc := NewCommentService(repositories.user, repositories.post, repositories.comment, cache, newTestCachePolicy(), newTestAuthorizationPolicy())
	postSvc := NewPostService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, cache, newTestCachePolicy(), newTestAuthorizationPolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	_, err := postSvc.GetPostDetail(postID)
	require.NoError(t, err)
	_, err = commentSvc.GetCommentsByPost(postID, 10, 0)
	require.NoError(t, err)

	_, err = commentSvc.CreateComment("new-comment", userID, postID)
	require.NoError(t, err)

	_, ok, err := cache.Get(key.PostDetail(postID))
	require.NoError(t, err)
	assert.False(t, ok)
	_, ok, err = cache.Get(key.CommentList(postID, 10, 0))
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCommentService_GetCommentsByPost_ReturnsCacheFailure_WhenCacheLoadFails(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, &errorCache{
		getOrSetWithTTLErr: newCacheFailure(nil),
	}, newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err := svc.GetCommentsByPost(1, 10, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrCacheFailure))
}
