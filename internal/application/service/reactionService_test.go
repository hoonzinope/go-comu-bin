package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/testutil"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReactionService_SetReaction_InvalidTargetType(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	svc := NewReactionService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())

	_, err := svc.SetReaction(context.Background(), userID, "target-uuid", model.ReactionTargetType("invalid"), model.ReactionTypeLike)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))
}

func TestReactionService_GetReactionsByTarget_AndDeleteByOwner(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	commentID := seedComment(repositories.comment, userID, postID, "comment")
	svc := NewReactionService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())

	created, err := svc.SetReaction(context.Background(), userID, mustCommentUUID(t, repositories.comment, commentID), model.ReactionTargetComment, model.ReactionTypeLike)
	require.NoError(t, err)
	assert.True(t, created)

	reactions, err := svc.GetReactionsByTarget(context.Background(), mustCommentUUID(t, repositories.comment, commentID), model.ReactionTargetComment)
	require.NoError(t, err)
	require.Len(t, reactions, 1)

	require.NoError(t, svc.DeleteReaction(context.Background(), userID, mustCommentUUID(t, repositories.comment, commentID), model.ReactionTargetComment))

	reactions, err = svc.GetReactionsByTarget(context.Background(), mustCommentUUID(t, repositories.comment, commentID), model.ReactionTargetComment)
	require.NoError(t, err)
	assert.Empty(t, reactions)
}

func TestReactionService_SetReaction_CreatesWhenMissing(t *testing.T) {
	repositories := newTestRepositories()
	cache := testutil.NewSpyCache()
	reactionSvc := NewReactionServiceWithActionDispatcher(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, cache, newTestActionDispatcher(t, repositories, cache), newTestCachePolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	_, err := reactionSvc.GetReactionsByTarget(context.Background(), mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost)
	require.NoError(t, err)
	_, ok, err := cache.Get(context.Background(), key.ReactionList("post", postID))
	require.NoError(t, err)
	require.True(t, ok)

	created, err := reactionSvc.SetReaction(context.Background(), userID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost, model.ReactionTypeLike)
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
	reactionSvc := NewReactionService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, &errorCache{
		deleteErr: newCacheFailure(nil),
	}, newTestCachePolicy())

	created, err := reactionSvc.SetReaction(context.Background(), userID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost, model.ReactionTypeLike)
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
	reactionSvc := NewReactionServiceWithActionDispatcher(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, cache, newTestActionDispatcher(t, repositories, cache), newTestCachePolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	created, err := reactionSvc.SetReaction(context.Background(), userID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost, model.ReactionTypeLike)
	require.NoError(t, err)
	assert.True(t, created)

	created, err = reactionSvc.SetReaction(context.Background(), userID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost, model.ReactionTypeDislike)
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
	reactionSvc := NewReactionServiceWithActionDispatcher(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, cache, newTestActionDispatcher(t, repositories, cache), newTestCachePolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	created, err := reactionSvc.SetReaction(context.Background(), userID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost, model.ReactionTypeLike)
	require.NoError(t, err)
	assert.True(t, created)

	created, err = reactionSvc.SetReaction(context.Background(), userID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost, model.ReactionTypeLike)
	require.NoError(t, err)
	assert.False(t, created)

	reactions, repoErr := repositories.reaction.GetByTarget(context.Background(), postID, entity.ReactionTargetPost)
	require.NoError(t, repoErr)
	require.Len(t, reactions, 1)
	assert.Equal(t, entity.ReactionTypeLike, reactions[0].Type)
}

func TestReactionService_DeleteReaction_NoOpWhenMissing(t *testing.T) {
	repositories := newTestRepositories()
	reactionSvc := NewReactionService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	require.NoError(t, reactionSvc.DeleteReaction(context.Background(), userID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost))
}

func TestReactionService_DeleteReaction_RemovesOwnedReaction(t *testing.T) {
	repositories := newTestRepositories()
	reactionSvc := NewReactionService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	created, err := reactionSvc.SetReaction(context.Background(), userID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost, model.ReactionTypeLike)
	require.NoError(t, err)
	assert.True(t, created)

	require.NoError(t, reactionSvc.DeleteReaction(context.Background(), userID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost))

	reactions, repoErr := repositories.reaction.GetByTarget(context.Background(), postID, entity.ReactionTargetPost)
	require.NoError(t, repoErr)
	assert.Empty(t, reactions)
}

func TestReactionService_DeleteReaction_DoesNotRemoveOtherUsersReaction(t *testing.T) {
	repositories := newTestRepositories()
	reactionSvc := NewReactionService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())

	ownerID := seedUser(repositories.user, "owner", "pw", "user")
	otherID := seedUser(repositories.user, "other", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")

	created, err := reactionSvc.SetReaction(context.Background(), ownerID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost, model.ReactionTypeLike)
	require.NoError(t, err)
	assert.True(t, created)

	require.NoError(t, reactionSvc.DeleteReaction(context.Background(), otherID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost))

	reactions, repoErr := repositories.reaction.GetByTarget(context.Background(), postID, entity.ReactionTargetPost)
	require.NoError(t, repoErr)
	require.Len(t, reactions, 1)
	assert.Equal(t, ownerID, reactions[0].UserID)
}

func TestReactionService_DeleteReaction_InvalidatesCommentAndPostCaches(t *testing.T) {
	repositories := newTestRepositories()
	cache := testutil.NewSpyCache()
	reactionSvc := NewReactionServiceWithActionDispatcher(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, cache, newTestActionDispatcher(t, repositories, cache), newTestCachePolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	commentID := seedComment(repositories.comment, userID, postID, "comment")

	created, err := reactionSvc.SetReaction(context.Background(), userID, mustCommentUUID(t, repositories.comment, commentID), model.ReactionTargetComment, model.ReactionTypeLike)
	require.NoError(t, err)
	assert.True(t, created)

	_, err = reactionSvc.GetReactionsByTarget(context.Background(), mustCommentUUID(t, repositories.comment, commentID), model.ReactionTargetComment)
	require.NoError(t, err)
	require.NoError(t, cache.Set(context.Background(), key.PostDetail(postID), "cached-post-detail"))

	require.NoError(t, reactionSvc.DeleteReaction(context.Background(), userID, mustCommentUUID(t, repositories.comment, commentID), model.ReactionTargetComment))

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
	svc := NewReactionService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, &errorCache{
		getOrSetWithTTLErr: newCacheFailure(nil),
	}, newTestCachePolicy())
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	_, err := svc.GetReactionsByTarget(context.Background(), mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrCacheFailure))
}

func TestReactionService_GetReactionsByTarget_ReturnsPostNotFound_WhenPostDeleted(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	postUUID := mustPostUUID(t, repositories.post, postID)
	svc := NewReactionService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())

	require.NoError(t, repositories.post.Delete(context.Background(), postID))

	_, err := svc.GetReactionsByTarget(context.Background(), postUUID, model.ReactionTargetPost)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrPostNotFound))
}

func TestReactionService_HiddenBoard_BlockedForNonAdmin(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	authorID := seedUser(repositories.user, "author", "pw", "user")
	boardID := seedBoard(repositories.board, "hidden", "desc")
	postID := seedPost(repositories.post, authorID, boardID, "title", "content")
	board, err := repositories.board.SelectBoardByID(context.Background(), boardID)
	require.NoError(t, err)
	require.NotNil(t, board)
	board.SetHidden(true)
	require.NoError(t, repositories.board.Update(context.Background(), board))
	svc := NewReactionService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())

	_, err = svc.GetReactionsByTarget(context.Background(), mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrBoardNotFound))

	_, err = svc.SetReaction(context.Background(), userID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost, model.ReactionTypeLike)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrBoardNotFound))
}

func TestReactionService_SetReaction_BlockedForGuestUser(t *testing.T) {
	repositories := newTestRepositories()
	reactionSvc := NewReactionService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())
	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "pw")
	guestID, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, guestID, boardID, "title", "content")

	_, err = reactionSvc.SetReaction(context.Background(), guestID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost, model.ReactionTypeLike)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrForbidden))
}

func TestReactionService_SetReaction_AllowsUnverifiedRegisteredUser(t *testing.T) {
	repositories := newTestRepositories()
	reactionSvc := NewReactionService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())
	user := entity.NewUserWithEmail("alice", "alice@example.com", "pw")
	userID, err := repositories.user.Save(context.Background(), user)
	require.NoError(t, err)
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	created, err := reactionSvc.SetReaction(context.Background(), userID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost, model.ReactionTypeLike)
	require.NoError(t, err)
	assert.True(t, created)
}
