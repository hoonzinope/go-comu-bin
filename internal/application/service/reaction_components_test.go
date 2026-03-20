package service

import (
	"context"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReactionQueryHandler_GetReactionsByTarget(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewReactionService(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, newTestCache(), newTestCachePolicy())
	_, err := svc.SetReaction(context.Background(), userID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost, model.ReactionTypeLike)
	require.NoError(t, err)

	handler := newReactionQueryHandler(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, newTestCache(), newTestCachePolicy())
	items, err := handler.GetReactionsByTarget(context.Background(), mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "like", string(items[0].Type))
}

func TestReactionCommandHandler_DeleteReaction_NoOpWhenMissing(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")

	queryHandler := newReactionQueryHandler(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, newTestCache(), newTestCachePolicy())
	handler := newReactionCommandHandler(repositories.user, repositories.board, repositories.post, repositories.comment, repositories.reaction, repositories.unitOfWork, resolveActionDispatcher(nil), queryHandler)
	err := handler.DeleteReaction(context.Background(), userID, mustPostUUID(t, repositories.post, postID), model.ReactionTargetPost)
	require.NoError(t, err)
}
