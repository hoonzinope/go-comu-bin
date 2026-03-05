package service

import (
	"errors"
	"testing"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostService_UpdatePost_ForbiddenForNonOwnerNonAdmin(t *testing.T) {
	repository := newTestRepository()
	ownerID := seedUser(repository, "owner", "pw", "user")
	otherID := seedUser(repository, "other", "pw", "user")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, ownerID, boardID, "title", "content")
	svc := NewPostService(repository)

	err := svc.UpdatePost(postID, otherID, "new-title", "new-content")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}

func TestPostService_UpdatePost_AllowedForAdmin(t *testing.T) {
	repository := newTestRepository()
	ownerID := seedUser(repository, "owner", "pw", "user")
	adminID := seedUser(repository, "admin", "pw", "admin")
	boardID := seedBoard(repository, "free", "desc")
	postID := seedPost(repository, ownerID, boardID, "title", "content")
	svc := NewPostService(repository)

	require.NoError(t, svc.UpdatePost(postID, adminID, "new-title", "new-content"))
}

func TestPostService_CreateGetListDelete_Success(t *testing.T) {
	repository := newTestRepository()
	userID := seedUser(repository, "user", "pw", "user")
	boardID := seedBoard(repository, "free", "desc")
	svc := NewPostService(repository)

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

func TestPostService_GetPostsList_HasMoreAndNextCursor(t *testing.T) {
	repository := newTestRepository()
	userID := seedUser(repository, "user", "pw", "user")
	boardID := seedBoard(repository, "free", "desc")
	seedPost(repository, userID, boardID, "title1", "content1")
	seedPost(repository, userID, boardID, "title2", "content2")
	seedPost(repository, userID, boardID, "title3", "content3")
	svc := NewPostService(repository)

	list, err := svc.GetPostsList(boardID, 2, 0)
	require.NoError(t, err)
	require.Len(t, list.Posts, 2)
	assert.True(t, list.HasMore)
	require.NotNil(t, list.NextLastID)
	assert.Equal(t, list.Posts[len(list.Posts)-1].ID, *list.NextLastID)
}
