package service

import (
	"errors"
	"testing"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommentService_UpdateComment_ForbiddenForNonOwnerNonAdmin(t *testing.T) {
	repository := newTestRepository()
	ownerID := seedUser(repository, "owner", "pw", "user")
	otherID := seedUser(repository, "other", "pw", "user")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, ownerID, boardID, "title", "content")
	svc := NewCommentService(repository)
	commentID, err := svc.CreateComment("comment", ownerID, postID)
	require.NoError(t, err)

	err = svc.UpdateComment(commentID, otherID, "new-comment")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}

func TestCommentService_UpdateComment_AllowedForAdmin(t *testing.T) {
	repository := newTestRepository()
	ownerID := seedUser(repository, "owner", "pw", "user")
	adminID := seedUser(repository, "admin", "pw", "admin")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, ownerID, boardID, "title", "content")
	svc := NewCommentService(repository)
	commentID, err := svc.CreateComment("comment", ownerID, postID)
	require.NoError(t, err)

	require.NoError(t, svc.UpdateComment(commentID, adminID, "new-comment"))
}

func TestCommentService_CreateGetDelete_Success(t *testing.T) {
	repository := newTestRepository()
	userID := seedUser(repository, "user", "pw", "user")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, userID, boardID, "title", "content")
	svc := NewCommentService(repository)

	commentID, err := svc.CreateComment("comment", userID, postID)
	require.NoError(t, err)
	assert.NotZero(t, commentID)

	list, err := svc.GetCommentsByPost(postID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, list.Comments, 1)

	require.NoError(t, svc.DeleteComment(commentID, userID))
}
