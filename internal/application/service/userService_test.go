package service

import (
	"errors"
	"testing"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserService_SignUp_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, repositories.post, repositories.comment, repositories.reaction, newTestPasswordHasher())

	result, err := svc.SignUp("alice", "pw")
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestUserService_SignUp_Duplicate(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, repositories.post, repositories.comment, repositories.reaction, newTestPasswordHasher())
	_, _ = svc.SignUp("alice", "pw")

	_, err := svc.SignUp("alice", "pw2")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserAlreadyExists))
}

func TestUserService_SignUp_InvalidInput(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, repositories.post, repositories.comment, repositories.reaction, newTestPasswordHasher())

	_, err := svc.SignUp(" ", "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}

func TestUserService_DeleteMe_InvalidCredential(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, repositories.post, repositories.comment, repositories.reaction, newTestPasswordHasher())
	_, _ = svc.SignUp("alice", "pw")
	user, err := repositories.user.SelectUserByUsername("alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	err = svc.DeleteMe(user.ID, "wrong")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidCredential))
}

func TestUserService_DeleteMe_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, repositories.post, repositories.comment, repositories.reaction, newTestPasswordHasher())
	_, _ = svc.SignUp("alice", "pw")
	user, err := repositories.user.SelectUserByUsername("alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	require.NoError(t, svc.DeleteMe(user.ID, "pw"))
}

func TestUserService_DeleteMe_UserNotFound(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, repositories.post, repositories.comment, repositories.reaction, newTestPasswordHasher())

	err := svc.DeleteMe(999, "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserNotFound))
}

func TestUserService_DeleteMe_BlockedWhenUserHasPosts(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, repositories.post, repositories.comment, repositories.reaction, newTestPasswordHasher())
	_, _ = svc.SignUp("alice", "pw")
	user, err := repositories.user.SelectUserByUsername("alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	boardID := seedBoard(repositories.board, "free", "desc")
	seedPost(repositories.post, user.ID, boardID, "title", "content")

	err = svc.DeleteMe(user.ID, "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserDeletionBlocked))
}

func TestUserService_DeleteMe_BlockedWhenUserHasCommentsOrReactions(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, repositories.post, repositories.comment, repositories.reaction, newTestPasswordHasher())
	_, _ = svc.SignUp("alice", "pw")
	_, _ = svc.SignUp("bob", "pw")
	alice, err := repositories.user.SelectUserByUsername("alice")
	require.NoError(t, err)
	require.NotNil(t, alice)
	bob, err := repositories.user.SelectUserByUsername("bob")
	require.NoError(t, err)
	require.NotNil(t, bob)

	boardID := seedBoard(repositories.board, "free", "desc")
	postID := seedPost(repositories.post, bob.ID, boardID, "title", "content")
	seedComment(repositories.comment, alice.ID, postID, "comment")
	_, _, _, err = repositories.reaction.SetUserTargetReaction(alice.ID, postID, entity.ReactionTargetPost, entity.ReactionTypeLike)
	require.NoError(t, err)

	err = svc.DeleteMe(alice.ID, "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserDeletionBlocked))
}

func TestUserService_VerifyCredentials_UserNotFound(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, repositories.post, repositories.comment, repositories.reaction, newTestPasswordHasher())

	_, err := svc.VerifyCredentials("nope", "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidCredential))
}

func TestUserService_VerifyCredentials_WrongPassword(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, repositories.post, repositories.comment, repositories.reaction, newTestPasswordHasher())
	_, _ = svc.SignUp("alice", "pw")

	_, err := svc.VerifyCredentials("alice", "wrong")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidCredential))
}
