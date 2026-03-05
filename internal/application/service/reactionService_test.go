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
	repository := newTestRepository()
	ownerID := seedUser(repository, "owner", "pw", "user")
	otherID := seedUser(repository, "other", "pw", "user")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, ownerID, boardID, "title", "content")
	svc := NewReactionService(repository, newTestCache(), newTestCachePolicy())

	require.NoError(t, svc.AddReaction(ownerID, postID, "post", "like"))
	reactions, err := repository.ReactionRepository.GetByTarget(postID, "post")
	require.NoError(t, err)
	require.Len(t, reactions, 1)

	err = svc.RemoveReaction(otherID, reactions[0].ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}

func TestReactionService_RemoveReaction_AllowedForAdmin(t *testing.T) {
	repository := newTestRepository()
	ownerID := seedUser(repository, "owner", "pw", "user")
	adminID := seedUser(repository, "admin", "pw", "admin")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, ownerID, boardID, "title", "content")
	svc := NewReactionService(repository, newTestCache(), newTestCachePolicy())

	require.NoError(t, svc.AddReaction(ownerID, postID, "post", "like"))
	reactions, err := repository.ReactionRepository.GetByTarget(postID, "post")
	require.NoError(t, err)
	require.Len(t, reactions, 1)

	require.NoError(t, svc.RemoveReaction(adminID, reactions[0].ID))
}

func TestReactionService_AddReaction_InvalidTargetType(t *testing.T) {
	repository := newTestRepository()
	userID := seedUser(repository, "user", "pw", "user")
	svc := NewReactionService(repository, newTestCache(), newTestCachePolicy())

	err := svc.AddReaction(userID, 1, "invalid", "like")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInternalServerError))
}

func TestReactionService_GetReactionsByTarget_AndOwnerDelete(t *testing.T) {
	repository := newTestRepository()
	userID := seedUser(repository, "user", "pw", "user")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, userID, boardID, "title", "content")
	commentID := seedComment(repository, userID, postID, "comment")
	svc := NewReactionService(repository, newTestCache(), newTestCachePolicy())

	require.NoError(t, svc.AddReaction(userID, commentID, "comment", "like"))
	reactions, err := svc.GetReactionsByTarget(commentID, "comment")
	require.NoError(t, err)
	require.Len(t, reactions, 1)

	require.NoError(t, svc.RemoveReaction(userID, reactions[0].ID))
}

func TestReactionService_AddReaction_InvalidatesReactionListCache(t *testing.T) {
	repo := newTestRepository()
	cache := testutil.NewSpyCache()
	reactionSvc := NewReactionService(repo, cache, newTestCachePolicy())

	userID := seedUser(repo, "alice", "pw", "user")
	boardID := seedBoard(repo, "free", "desc")
	postID := seedPost(repo, userID, boardID, "title", "content")

	_, err := reactionSvc.GetReactionsByTarget(postID, "post")
	require.NoError(t, err)
	_, ok := cache.Get(key.ReactionList("post", postID))
	require.True(t, ok)

	require.NoError(t, reactionSvc.AddReaction(userID, postID, "post", "like"))

	_, ok = cache.Get(key.ReactionList("post", postID))
	assert.False(t, ok)
}
