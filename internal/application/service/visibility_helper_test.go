package service

import (
	"context"
	"errors"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsurePostVisibleForUser_HiddenBoardReturnsConfiguredNotFound(t *testing.T) {
	repositories := newTestRepositories()
	authorID := seedUser(repositories.user, "author", "pw", "user")
	boardID := seedBoard(repositories.board, "hidden", "desc")
	postID := seedPost(repositories.post, authorID, boardID, "title", "content")
	board, err := repositories.board.SelectBoardByID(context.Background(), boardID)
	require.NoError(t, err)
	require.NotNil(t, board)
	board.SetHidden(true)
	require.NoError(t, repositories.board.Update(context.Background(), board))

	post, err := policy.EnsurePostVisibleForUser(context.Background(), repositories.post, repositories.board, nil, postID, customerror.ErrPostNotFound, "test")
	require.Error(t, err)
	assert.Nil(t, post)
	assert.True(t, errors.Is(err, customerror.ErrPostNotFound))
}

func TestEnsureCommentTargetVisibleForUser_HiddenBoardReturnsConfiguredNotFound(t *testing.T) {
	repositories := newTestRepositories()
	authorID := seedUser(repositories.user, "author", "pw", "user")
	boardID := seedBoard(repositories.board, "hidden", "desc")
	postID := seedPost(repositories.post, authorID, boardID, "title", "content")
	commentID := seedComment(repositories.comment, authorID, postID, "comment")
	board, err := repositories.board.SelectBoardByID(context.Background(), boardID)
	require.NoError(t, err)
	require.NotNil(t, board)
	board.SetHidden(true)
	require.NoError(t, repositories.board.Update(context.Background(), board))

	comment, post, err := policy.EnsureCommentTargetVisibleForUser(context.Background(), repositories.comment, repositories.post, repositories.board, nil, commentID, customerror.ErrCommentNotFound, "test")
	require.Error(t, err)
	assert.Nil(t, comment)
	assert.Nil(t, post)
	assert.True(t, errors.Is(err, customerror.ErrCommentNotFound))
}
