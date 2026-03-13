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

func TestReactionService_SetReaction_InvalidTargetType(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	svc := NewReactionService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())

	_, err := svc.SetReaction(context.Background(), userID, 1, entity.ReactionTargetType("invalid"), entity.ReactionTypeLike)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInternalServerError))
}

func TestReactionService_GetReactionsByTarget_AndDeleteByOwner(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	commentID := seedComment(repositories.comment, userID, postID, "comment")
	svc := NewReactionService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())

	created, err := svc.SetReaction(context.Background(), userID, commentID, entity.ReactionTargetComment, entity.ReactionTypeLike)
	require.NoError(t, err)
	assert.True(t, created)

	reactions, err := svc.GetReactionsByTarget(context.Background(), commentID, entity.ReactionTargetComment)
	require.NoError(t, err)
	require.Len(t, reactions, 1)

	require.NoError(t, svc.DeleteReaction(context.Background(), userID, commentID, entity.ReactionTargetComment))

	reactions, err = svc.GetReactionsByTarget(context.Background(), commentID, entity.ReactionTargetComment)
	require.NoError(t, err)
	assert.Empty(t, reactions)
}

func TestReactionService_SetReaction_CreatesWhenMissing(t *testing.T) {
	repositories := newTestRepositories()
	cache := testutil.NewSpyCache()
	reactionSvc := NewReactionServiceWithPublisher(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, cache, newTestEventPublisher(t, repositories, cache), newTestCachePolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	_, err := reactionSvc.GetReactionsByTarget(context.Background(), postID, entity.ReactionTargetPost)
	require.NoError(t, err)
	_, ok, err := cache.Get(context.Background(), key.ReactionList("post", postID))
	require.NoError(t, err)
	require.True(t, ok)

	created, err := reactionSvc.SetReaction(context.Background(), userID, postID, entity.ReactionTargetPost, entity.ReactionTypeLike)
	require.NoError(t, err)
	assert.True(t, created)

	require.Eventually(t, func() bool {
		_, ok, err = cache.Get(context.Background(), key.ReactionList("post", postID))
		require.NoError(t, err)
		return !ok
	}, time.Second, 10*time.Millisecond)

	reactions, repoErr := repositories.reaction.GetByTarget(context.Background(), postID, entity.ReactionTargetPost)
	require.NoError(t, repoErr)
	require.Len(t, reactions, 1)
	assert.Equal(t, entity.ReactionTypeLike, reactions[0].Type)
}

func TestReactionService_SetReaction_SucceedsWhenCacheInvalidationFails(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	reactionSvc := NewReactionService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, &errorCache{
		deleteErr: newCacheFailure(nil),
	}, newTestCachePolicy())

	created, err := reactionSvc.SetReaction(context.Background(), userID, postID, entity.ReactionTargetPost, entity.ReactionTypeLike)
	require.NoError(t, err)
	assert.True(t, created)

	reactions, repoErr := repositories.reaction.GetByTarget(context.Background(), postID, entity.ReactionTargetPost)
	require.NoError(t, repoErr)
	require.Len(t, reactions, 1)
	assert.Equal(t, entity.ReactionTypeLike, reactions[0].Type)
}

func TestReactionService_SetReaction_UpdatesExistingType(t *testing.T) {
	repositories := newTestRepositories()
	cache := testutil.NewSpyCache()
	reactionSvc := NewReactionServiceWithPublisher(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, cache, newTestEventPublisher(t, repositories, cache), newTestCachePolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	created, err := reactionSvc.SetReaction(context.Background(), userID, postID, entity.ReactionTargetPost, entity.ReactionTypeLike)
	require.NoError(t, err)
	assert.True(t, created)

	created, err = reactionSvc.SetReaction(context.Background(), userID, postID, entity.ReactionTargetPost, entity.ReactionTypeDislike)
	require.NoError(t, err)
	assert.False(t, created)

	reactions, repoErr := repositories.reaction.GetByTarget(context.Background(), postID, entity.ReactionTargetPost)
	require.NoError(t, repoErr)
	require.Len(t, reactions, 1)
	assert.Equal(t, entity.ReactionTypeDislike, reactions[0].Type)
}

func TestReactionService_SetReaction_NoOpWhenSameType(t *testing.T) {
	repositories := newTestRepositories()
	cache := testutil.NewSpyCache()
	reactionSvc := NewReactionServiceWithPublisher(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, cache, newTestEventPublisher(t, repositories, cache), newTestCachePolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	created, err := reactionSvc.SetReaction(context.Background(), userID, postID, entity.ReactionTargetPost, entity.ReactionTypeLike)
	require.NoError(t, err)
	assert.True(t, created)

	created, err = reactionSvc.SetReaction(context.Background(), userID, postID, entity.ReactionTargetPost, entity.ReactionTypeLike)
	require.NoError(t, err)
	assert.False(t, created)

	reactions, repoErr := repositories.reaction.GetByTarget(context.Background(), postID, entity.ReactionTargetPost)
	require.NoError(t, repoErr)
	require.Len(t, reactions, 1)
	assert.Equal(t, entity.ReactionTypeLike, reactions[0].Type)
}

func TestReactionService_DeleteReaction_NoOpWhenMissing(t *testing.T) {
	repositories := newTestRepositories()
	reactionSvc := NewReactionService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	require.NoError(t, reactionSvc.DeleteReaction(context.Background(), userID, postID, entity.ReactionTargetPost))
}

func TestReactionService_DeleteReaction_RemovesOwnedReaction(t *testing.T) {
	repositories := newTestRepositories()
	reactionSvc := NewReactionService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	created, err := reactionSvc.SetReaction(context.Background(), userID, postID, entity.ReactionTargetPost, entity.ReactionTypeLike)
	require.NoError(t, err)
	assert.True(t, created)

	require.NoError(t, reactionSvc.DeleteReaction(context.Background(), userID, postID, entity.ReactionTargetPost))

	reactions, repoErr := repositories.reaction.GetByTarget(context.Background(), postID, entity.ReactionTargetPost)
	require.NoError(t, repoErr)
	assert.Empty(t, reactions)
}

func TestReactionService_DeleteReaction_DoesNotRemoveOtherUsersReaction(t *testing.T) {
	repositories := newTestRepositories()
	reactionSvc := NewReactionService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())

	ownerID := seedUser(repositories.user, "owner", "pw", "user")
	otherID := seedUser(repositories.user, "other", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")

	created, err := reactionSvc.SetReaction(context.Background(), ownerID, postID, entity.ReactionTargetPost, entity.ReactionTypeLike)
	require.NoError(t, err)
	assert.True(t, created)

	require.NoError(t, reactionSvc.DeleteReaction(context.Background(), otherID, postID, entity.ReactionTargetPost))

	reactions, repoErr := repositories.reaction.GetByTarget(context.Background(), postID, entity.ReactionTargetPost)
	require.NoError(t, repoErr)
	require.Len(t, reactions, 1)
	assert.Equal(t, ownerID, reactions[0].UserID)
}

func TestReactionService_DeleteReaction_InvalidatesCommentAndPostCaches(t *testing.T) {
	repositories := newTestRepositories()
	cache := testutil.NewSpyCache()
	reactionSvc := NewReactionServiceWithPublisher(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, cache, newTestEventPublisher(t, repositories, cache), newTestCachePolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	commentID := seedComment(repositories.comment, userID, postID, "comment")

	created, err := reactionSvc.SetReaction(context.Background(), userID, commentID, entity.ReactionTargetComment, entity.ReactionTypeLike)
	require.NoError(t, err)
	assert.True(t, created)

	_, err = reactionSvc.GetReactionsByTarget(context.Background(), commentID, entity.ReactionTargetComment)
	require.NoError(t, err)
	require.NoError(t, cache.Set(context.Background(), key.PostDetail(postID), "cached-post-detail"))

	require.NoError(t, reactionSvc.DeleteReaction(context.Background(), userID, commentID, entity.ReactionTargetComment))

	require.Eventually(t, func() bool {
		_, ok, err := cache.Get(context.Background(), key.ReactionList("comment", commentID))
		require.NoError(t, err)
		if ok {
			return false
		}
		_, ok, err = cache.Get(context.Background(), key.PostDetail(postID))
		require.NoError(t, err)
		return !ok
	}, time.Second, 10*time.Millisecond)
}

func TestReactionService_GetReactionsByTarget_ReturnsCacheFailure_WhenCacheLoadFails(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewReactionService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, &errorCache{
		getOrSetWithTTLErr: newCacheFailure(nil),
	}, newTestCachePolicy())

	_, err := svc.GetReactionsByTarget(context.Background(), 1, entity.ReactionTargetPost)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrCacheFailure))
}

func TestReactionService_GetReactionsByTarget_ReturnsPostNotFound_WhenPostDeleted(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewReactionService(repositories.user, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())

	require.NoError(t, repositories.post.Delete(context.Background(), postID))

	_, err := svc.GetReactionsByTarget(context.Background(), postID, entity.ReactionTargetPost)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrPostNotFound))
}
