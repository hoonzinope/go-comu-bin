package service

import (
	"errors"
	"testing"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserService_SignUp_Success(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher())

	result, err := svc.SignUp("alice", "pw")
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestUserService_SignUp_Duplicate(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher())
	_, _ = svc.SignUp("alice", "pw")

	_, err := svc.SignUp("alice", "pw2")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserAlreadyExists))
}

func TestUserService_DeleteMe_InvalidCredential(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher())
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
	svc := NewUserService(repositories.user, newTestPasswordHasher())
	_, _ = svc.SignUp("alice", "pw")
	user, err := repositories.user.SelectUserByUsername("alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	require.NoError(t, svc.DeleteMe(user.ID, "pw"))
}

func TestUserService_DeleteMe_UserNotFound(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher())

	err := svc.DeleteMe(999, "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserNotFound))
}

func TestUserService_VerifyCredentials_UserNotFound(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher())

	_, err := svc.VerifyCredentials("nope", "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserNotFound))
}

func TestUserService_VerifyCredentials_WrongPassword(t *testing.T) {
	repositories := newTestRepositories()
	svc := NewUserService(repositories.user, newTestPasswordHasher())
	_, _ = svc.SignUp("alice", "pw")

	_, err := svc.VerifyCredentials("alice", "wrong")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidCredential))
}
