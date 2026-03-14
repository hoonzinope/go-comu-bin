package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/testutil"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommentService_UpdateComment_ForbiddenForNonOwnerNonAdmin(t *testing.T) {
	repositories := newTestRepositories()
	ownerID := seedUser(repositories.user, "owner", "pw", "user")
	otherID := seedUser(repositories.user, "other", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())
	commentID, err := svc.CreateComment(context.Background(), "comment", ownerID, postID, nil)
	require.NoError(t, err)

	err = svc.UpdateComment(context.Background(), commentID, otherID, "new-comment")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}

func TestCommentService_UpdateComment_AllowedForAdmin(t *testing.T) {
	repositories := newTestRepositories()
	ownerID := seedUser(repositories.user, "owner", "pw", "user")
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())
	commentID, err := svc.CreateComment(context.Background(), "comment", ownerID, postID, nil)
	require.NoError(t, err)

	require.NoError(t, svc.UpdateComment(context.Background(), commentID, adminID, "new-comment"))
}

func TestCommentService_CreateGetDelete_Success(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	commentID, err := svc.CreateComment(context.Background(), "comment", userID, postID, nil)
	require.NoError(t, err)
	assert.NotZero(t, commentID)

	list, err := svc.GetCommentsByPost(context.Background(), postID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, list.Comments, 1)

	require.NoError(t, svc.DeleteComment(context.Background(), commentID, userID))
}

func TestCommentService_CreateComment_InvalidInput(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err := svc.CreateComment(context.Background(), " ", userID, postID, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}

func TestCommentService_CreateComment_BlockedForSuspendedUser(t *testing.T) {
	repositories := newTestRepositories()
	user := entity.NewUser("user", "pw")
	user.Suspend("spam", nil)
	userID, err := repositories.user.Save(context.Background(), user)
	require.NoError(t, err)
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err = svc.CreateComment(context.Background(), "comment", userID, postID, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserSuspended))
}

func TestCommentService_GetCommentsByPost_HasMoreAndNextCursor(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	seedComment(repositories.comment, userID, postID, "c1")
	seedComment(repositories.comment, userID, postID, "c2")
	seedComment(repositories.comment, userID, postID, "c3")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	list, err := svc.GetCommentsByPost(context.Background(), postID, 2, 0)
	require.NoError(t, err)
	require.Len(t, list.Comments, 2)
	assert.True(t, list.HasMore)
	require.NotNil(t, list.NextLastID)
	assert.Equal(t, list.Comments[len(list.Comments)-1].ID, *list.NextLastID)
}

func TestCommentService_GetCommentsByPost_InvalidLimit(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	seedComment(repositories.comment, userID, postID, "c1")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err := svc.GetCommentsByPost(context.Background(), postID, 0, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}

func TestCommentService_GetCommentsByPost_ReturnsPostNotFound_WhenPostDeleted(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	seedComment(repositories.comment, userID, postID, "c1")
	require.NoError(t, repositories.post.Delete(context.Background(), postID))
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err := svc.GetCommentsByPost(context.Background(), postID, 10, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrPostNotFound))
}

func TestCommentService_CreateComment_InvalidatesRelatedCaches(t *testing.T) {
	repositories := newTestRepositories()
	cache := testutil.NewSpyCache()
	commentSvc := NewCommentServiceWithActionDispatcher(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, cache, newTestActionDispatcher(t, repositories, cache), newTestCachePolicy(), newTestAuthorizationPolicy())
	postSvc := newTestPostService(t, repositories, cache)

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	_, err := postSvc.GetPostDetail(context.Background(), postID)
	require.NoError(t, err)
	_, err = commentSvc.GetCommentsByPost(context.Background(), postID, 10, 0)
	require.NoError(t, err)

	_, err = commentSvc.CreateComment(context.Background(), "new-comment", userID, postID, nil)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, ok, err := cache.Get(context.Background(), key.PostDetail(postID))
		require.NoError(t, err)
		if ok {
			return false
		}
		_, ok, err = cache.Get(context.Background(), key.CommentList(postID, 10, 0))
		require.NoError(t, err)
		return !ok
	}, time.Second, 10*time.Millisecond)
}

func TestCommentService_CreateComment_SucceedsWhenCacheInvalidationFails(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, &errorCache{
		deleteErr:         newCacheFailure(nil),
		deleteByPrefixErr: newCacheFailure(nil),
	}, newTestCachePolicy(), newTestAuthorizationPolicy())

	commentID, err := svc.CreateComment(context.Background(), "new-comment", userID, postID, nil)
	require.NoError(t, err)
	assert.NotZero(t, commentID)

	comment, repoErr := repositories.comment.SelectCommentByID(context.Background(), commentID)
	require.NoError(t, repoErr)
	require.NotNil(t, comment)
	assert.Equal(t, "new-comment", comment.Content)
}

func TestCommentService_GetCommentsByPost_ReturnsCacheFailure_WhenCacheLoadFails(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, &errorCache{
		getOrSetWithTTLErr: newCacheFailure(nil),
	}, newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err := svc.GetCommentsByPost(context.Background(), 1, 10, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrCacheFailure))
}

func TestCommentService_DeleteComment_SoftDeletedCommentIsNoLongerVisible(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	commentID, err := svc.CreateComment(context.Background(), "comment", userID, postID, nil)
	require.NoError(t, err)

	require.NoError(t, svc.DeleteComment(context.Background(), commentID, userID))

	list, err := svc.GetCommentsByPost(context.Background(), postID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, list.Comments)
}

func TestCommentService_DeleteComment_RemovesStoredReactions(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	commentID, err := svc.CreateComment(context.Background(), "comment", userID, postID, nil)
	require.NoError(t, err)
	_, _, _, err = repositories.reaction.SetUserTargetReaction(context.Background(), userID, commentID, entity.ReactionTargetComment, entity.ReactionTypeLike)
	require.NoError(t, err)

	require.NoError(t, svc.DeleteComment(context.Background(), commentID, userID))

	reactions, err := repositories.reaction.GetByTarget(context.Background(), commentID, entity.ReactionTargetComment)
	require.NoError(t, err)
	assert.Empty(t, reactions)
}

func TestCommentService_DeleteComment_SoftDeletesReplyComments(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	parentID, err := svc.CreateComment(context.Background(), "parent", userID, postID, nil)
	require.NoError(t, err)
	replyID, err := svc.CreateComment(context.Background(), "reply", userID, postID, &parentID)
	require.NoError(t, err)

	require.NoError(t, svc.DeleteComment(context.Background(), parentID, userID))

	list, err := svc.GetCommentsByPost(context.Background(), postID, 10, 0)
	require.NoError(t, err)
	require.Len(t, list.Comments, 2)
	assert.Equal(t, parentID, list.Comments[1].ID)
	assert.Equal(t, "삭제된 댓글", list.Comments[1].Content)
	assert.Equal(t, replyID, list.Comments[0].ID)
	assert.Equal(t, "reply", list.Comments[0].Content)

	reply, err := repositories.comment.SelectCommentByID(context.Background(), replyID)
	require.NoError(t, err)
	require.NotNil(t, reply)
	assert.Equal(t, entity.CommentStatusActive, reply.Status)
}

func TestCommentService_CreateReplyComment_Success(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	parentID := seedComment(repositories.comment, userID, postID, "parent")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	commentID, err := svc.CreateComment(context.Background(), "reply", userID, postID, &parentID)
	require.NoError(t, err)
	assert.NotZero(t, commentID)

	list, err := svc.GetCommentsByPost(context.Background(), postID, 10, 0)
	require.NoError(t, err)
	require.Len(t, list.Comments, 2)
	assert.Equal(t, &parentID, list.Comments[0].ParentID)
}

func TestCommentService_CreateReplyComment_RejectsNestedReply(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	parentID := seedComment(repositories.comment, userID, postID, "parent")
	replyID := seedCommentWithParent(repositories.comment, userID, postID, "reply", &parentID)
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err := svc.CreateComment(context.Background(), "nested", userID, postID, &replyID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}

func TestCommentService_CreateReplyComment_RejectsParentFromAnotherPost(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	otherPostID := seedPost(repositories.post, userID, boardID, "title2", "content2")
	parentID := seedComment(repositories.comment, userID, otherPostID, "parent")
	svc := NewCommentService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	_, err := svc.CreateComment(context.Background(), "reply", userID, postID, &parentID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}
