package service

import (
	"errors"
	"testing"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserService_SignUp_Success(t *testing.T) {
	repository := newTestRepository()
	svc := NewUserService(repository)

	result, err := svc.SignUp("alice", "pw")
	require.NoError(t, err)
	assert.Equal(t, "ok", result)
}

func TestUserService_SignUp_Duplicate(t *testing.T) {
	repository := newTestRepository()
	svc := NewUserService(repository)
	_, _ = svc.SignUp("alice", "pw")

	_, err := svc.SignUp("alice", "pw2")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserAlreadyExists))
}

func TestUserService_Quit_InvalidCredential(t *testing.T) {
	repository := newTestRepository()
	svc := NewUserService(repository)
	_, _ = svc.SignUp("alice", "pw")

	err := svc.Quit("alice", "wrong")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidCredential))
}

func TestUserService_Quit_Success(t *testing.T) {
	repository := newTestRepository()
	svc := NewUserService(repository)
	_, _ = svc.SignUp("alice", "pw")

	require.NoError(t, svc.Quit("alice", "pw"))
}

func TestUserService_Quit_UserNotFound(t *testing.T) {
	repository := newTestRepository()
	svc := NewUserService(repository)

	err := svc.Quit("nope", "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserNotFound))
}

func TestUserService_Login_UserNotFound(t *testing.T) {
	repository := newTestRepository()
	svc := NewUserService(repository)

	_, err := svc.Login("nope", "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserNotFound))
}

func TestUserService_Login_WrongPassword(t *testing.T) {
	repository := newTestRepository()
	svc := NewUserService(repository)
	_, _ = svc.SignUp("alice", "pw")

	_, err := svc.Login("alice", "wrong")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrUserNotFound))
}
