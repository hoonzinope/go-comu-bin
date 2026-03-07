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

func TestReactionService_RemoveReaction_ForbiddenForNonOwnerNonAdmin(t *testing.T) {
	repositories := newTestRepositories()
	ownerID := seedUser(repositories.user, "owner", "pw", "user")
	otherID := seedUser(repositories.user, "other", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")
	svc := NewReactionService(repositories.user, repositories.post, repositories.comment, repositories.reaction, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	require.NoError(t, svc.AddReaction(ownerID, postID, "post", "like"))
	reactions, err := repositories.reaction.GetByTarget(postID, "post")
	require.NoError(t, err)
	require.Len(t, reactions, 1)

	err = svc.RemoveReaction(otherID, reactions[0].ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}

func TestReactionService_RemoveReaction_AllowedForAdmin(t *testing.T) {
	repositories := newTestRepositories()
	ownerID := seedUser(repositories.user, "owner", "pw", "user")
	adminID := seedUser(repositories.user, "admin", "pw", "admin")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")
	svc := NewReactionService(repositories.user, repositories.post, repositories.comment, repositories.reaction, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	require.NoError(t, svc.AddReaction(ownerID, postID, "post", "like"))
	reactions, err := repositories.reaction.GetByTarget(postID, "post")
	require.NoError(t, err)
	require.Len(t, reactions, 1)

	require.NoError(t, svc.RemoveReaction(adminID, reactions[0].ID))
}

func TestReactionService_AddReaction_InvalidTargetType(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	svc := NewReactionService(repositories.user, repositories.post, repositories.comment, repositories.reaction, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	err := svc.AddReaction(userID, 1, "invalid", "like")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInternalServerError))
}

func TestReactionService_GetReactionsByTarget_AndOwnerDelete(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "user", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	commentID := seedComment(repositories.comment, userID, postID, "comment")
	svc := NewReactionService(repositories.user, repositories.post, repositories.comment, repositories.reaction, newTestCache(), newTestCachePolicy(), newTestAuthorizationPolicy())

	require.NoError(t, svc.AddReaction(userID, commentID, "comment", "like"))
	reactions, err := svc.GetReactionsByTarget(commentID, "comment")
	require.NoError(t, err)
	require.Len(t, reactions, 1)

	require.NoError(t, svc.RemoveReaction(userID, reactions[0].ID))
}

func TestReactionService_AddReaction_InvalidatesReactionListCache(t *testing.T) {
	repositories := newTestRepositories()
	cache := testutil.NewSpyCache()
	reactionSvc := NewReactionService(repositories.user, repositories.post, repositories.comment, repositories.reaction, cache, newTestCachePolicy(), newTestAuthorizationPolicy())

	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	_, err := reactionSvc.GetReactionsByTarget(postID, "post")
	require.NoError(t, err)
	_, ok := cache.Get(key.ReactionList("post", postID))
	require.True(t, ok)

	require.NoError(t, reactionSvc.AddReaction(userID, postID, "post", "like"))

	_, ok = cache.Get(key.ReactionList("post", postID))
	assert.False(t, ok)
}
