package service

import (
	"errors"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	cacheInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

func TestSessionService_Login_Success(t *testing.T) {
	repository := newTestRepository()
	userService := NewUserService(repository)
	_, err := userService.SignUp("alice", "pw")
	require.NoError(t, err)

	cache := cacheInMemory.NewInMemoryCache()
	svc := NewSessionService(userService, auth.NewJwtTokenProvider("test-secret"), cache)

	token, err := svc.Login("alice", "pw")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	_, exists := cache.Get(token)
	assert.True(t, exists)
}

func TestSessionService_ValidateTokenToId_InvalidatedToken(t *testing.T) {
	repository := newTestRepository()
	userService := NewUserService(repository)
	_, err := userService.SignUp("alice", "pw")
	require.NoError(t, err)

	cache := cacheInMemory.NewInMemoryCache()
	svc := NewSessionService(userService, auth.NewJwtTokenProvider("test-secret"), cache)

	token, err := svc.Login("alice", "pw")
	require.NoError(t, err)
	require.NoError(t, svc.Logout(token))

	_, err = svc.ValidateTokenToId(token)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidToken))
}

func TestSessionService_ValidateTokenToId_Success(t *testing.T) {
	repository := newTestRepository()
	userID := seedUser(repository, "alice", "pw", "user")
	userService := NewUserService(repository)

	cache := cacheInMemory.NewInMemoryCache()
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(userID)
	require.NoError(t, err)
	cache.Set(token, userID)

	svc := NewSessionService(userService, tokenProvider, cache)

	gotUserID, err := svc.ValidateTokenToId(token)
	require.NoError(t, err)
	assert.Equal(t, userID, gotUserID)
}
