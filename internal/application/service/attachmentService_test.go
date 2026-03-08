package service

import (
	"errors"
	"testing"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachmentService_CreatePostAttachment_Success(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.post, repositories.attachment, newTestAuthorizationPolicy())

	id, err := svc.CreatePostAttachment(postID, userID, "a.png", "image/png", 10, "attachments/a.png")
	require.NoError(t, err)
	assert.NotZero(t, id)
}

func TestAttachmentService_GetPostAttachments_RequiresPublishedPost(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	post := seedDraftPost(repositories.post, userID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.post, repositories.attachment, newTestAuthorizationPolicy())

	_, err := svc.GetPostAttachments(post)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrPostNotFound))
}

func TestAttachmentService_DeletePostAttachment_ForbiddenForNonOwner(t *testing.T) {
	repositories := newTestRepositories()
	ownerID := seedUser(repositories.user, "alice", "pw", "user")
	otherID := seedUser(repositories.user, "bob", "pw", "user")
	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, ownerID, boardID, "title", "content")
	svc := NewAttachmentService(repositories.user, repositories.post, repositories.attachment, newTestAuthorizationPolicy())
	attachmentID, err := svc.CreatePostAttachment(postID, ownerID, "a.png", "image/png", 10, "attachments/a.png")
	require.NoError(t, err)

	err = svc.DeletePostAttachment(postID, attachmentID, otherID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrForbidden))
}
