package service

import (
	"errors"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/testutil"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostService_UpdatePost_ForbiddenForNonOwnerNonAdmin(t *testing.T) {
	repositories := newTestRepositories()
	ownerID := seedUser(repositories.user, "owner", "pw", "user")
	otherID := seedUser(repositories.user, "other", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")
	svc := NewPostService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	err := svc.UpdatePost(postID, otherID, "new-title", "new-content")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}

func TestPostService_UpdatePost_AllowedForAdmin(t *testing.T) {
	repositories := newTestRepositories()
	ownerID := seedUser(repositories.user, "owner", "pw", "user")
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")
	svc := NewPostService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	require.NoError(t, svc.UpdatePost(postID, adminID, "new-title", "new-content"))
}

func TestPostService_CreateGetListDelete_Success(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := NewPostService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	postID, err := svc.CreatePost("title", "content", userID, boardID)
	require.NoError(t, err)
	assert.NotZero(t, postID)

	list, err := svc.GetPostsList(boardID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, list.Posts, 1)

	detail, err := svc.GetPostDetail(postID)
	require.NoError(t, err)
	require.NotNil(t, detail.Post)
	assert.Equal(t, postID, detail.Post.ID)

	require.NoError(t, svc.DeletePost(postID, userID))
}

func TestPostService_CreatePost_InvalidInput(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := NewPostService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err := svc.CreatePost(" ", "content", userID, boardID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}

func TestPostService_CreatePost_BlockedForSuspendedUser(t *testing.T) {
	repositories := newTestRepositories()
	user := entity.NewUser("user", "pw")
	user.Suspend("spam", nil)
	userID, err := repositories.user.Save(user)
	require.NoError(t, err)
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := NewPostService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err = svc.CreatePost("title", "content", userID, boardID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserSuspended))
}

func TestPostService_GetPostsList_HasMoreAndNextCursor(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	seedPost(repositories.post, userID, boardID, "title1", "content1")
	seedPost(repositories.post, userID, boardID, "title2", "content2")
	seedPost(repositories.post, userID, boardID, "title3", "content3")
	svc := NewPostService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	list, err := svc.GetPostsList(boardID, 2, 0)
	require.NoError(t, err)
	require.Len(t, list.Posts, 2)
	assert.True(t, list.HasMore)
	require.NotNil(t, list.NextLastID)
	assert.Equal(t, list.Posts[len(list.Posts)-1].ID, *list.NextLastID)
}

func TestPostService_GetPostDetail_UsesCache(t *testing.T) {
	repositories := newTestRepositories()
	cache := testutil.NewSpyCache()
	postSvc := NewPostService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, cache, newTestCachePolicy(), newTestAuthorizationPolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	detail1, err := postSvc.GetPostDetail(postID)
	require.NoError(t, err)
	require.NotNil(t, detail1.Post)
	assert.Equal(t, "title", detail1.Post.Title)

	detail2, err := postSvc.GetPostDetail(postID)
	require.NoError(t, err)
	require.NotNil(t, detail2.Post)
	assert.Equal(t, "title", detail2.Post.Title)

	assert.Equal(t, 1, cache.LoadCount(key.PostDetail(postID)))
}

func TestPostService_UpdatePost_InvalidatesCaches(t *testing.T) {
	repositories := newTestRepositories()
	cache := testutil.NewSpyCache()
	postSvc := NewPostService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, cache, newTestCachePolicy(), newTestAuthorizationPolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	_, err := postSvc.GetPostDetail(postID)
	require.NoError(t, err)
	_, err = postSvc.GetPostsList(boardID, 10, 0)
	require.NoError(t, err)

	require.NoError(t, postSvc.UpdatePost(postID, userID, "new", "new-content"))

	_, ok, err := cache.Get(key.PostDetail(postID))
	require.NoError(t, err)
	assert.False(t, ok)
	_, ok, err = cache.Get(key.PostList(boardID, 10, 0))
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPostService_GetPostDetail_ReturnsCacheFailure_WhenCacheLoadFails(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewPostService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, &errorCache{
		getOrSetWithTTLErr: newCacheFailure(nil),
	}, newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err := svc.GetPostDetail(1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrCacheFailure))
}

func TestPostService_DeletePost_SoftDeletedPostIsNoLongerVisible(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewPostService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	require.NoError(t, svc.DeletePost(postID, userID))

	_, err := svc.GetPostDetail(postID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrPostNotFound))

	list, err := svc.GetPostsList(boardID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, list.Posts)
}

func TestPostService_CreateDraftPost_DoesNotAppearInPublicList(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := NewPostService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	postID, err := svc.CreateDraftPost("draft-title", "draft-content", userID, boardID)
	require.NoError(t, err)
	assert.NotZero(t, postID)

	list, err := svc.GetPostsList(boardID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, list.Posts)
}

func TestPostService_PublishPost_MakesDraftVisible(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	svc := NewPostService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	postID, err := svc.CreateDraftPost("draft-title", "draft-content", userID, boardID)
	require.NoError(t, err)

	err = svc.PublishPost(postID, userID)
	require.NoError(t, err)

	list, err := svc.GetPostsList(boardID, 10, 0)
	require.NoError(t, err)
	require.Len(t, list.Posts, 1)
	assert.Equal(t, postID, list.Posts[0].ID)
}
