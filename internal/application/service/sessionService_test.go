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
	repositories := newTestRepositories()
	userService := NewUserService(repositories.user, newTestPasswordHasher())
	_, err := userService.SignUp("alice", "pw")
	require.NoError(t, err)

	cache := cacheInMemory.NewInMemoryCache()
	svc := NewSessionService(userService, auth.NewJwtTokenProvider("test-secret"), cache)

	token, err := svc.Login("alice", "pw")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	user, err := repositories.user.SelectUserByUsername("alice")
	require.NoError(t, err)
	require.NotNil(t, user)
	_, exists := cache.Get(sessionCacheKey(user.ID, token))
	assert.True(t, exists)
}

func TestSessionService_ValidateTokenToId_InvalidatedToken(t *testing.T) {
	repositories := newTestRepositories()
	userService := NewUserService(repositories.user, newTestPasswordHasher())
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
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	userService := NewUserService(repositories.user, newTestPasswordHasher())

	cache := cacheInMemory.NewInMemoryCache()
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(userID)
	require.NoError(t, err)
	cache.SetWithTTL(sessionCacheKey(userID, token), userID, tokenProvider.TTLSeconds())

	svc := NewSessionService(userService, tokenProvider, cache)

	gotUserID, err := svc.ValidateTokenToId(token)
	require.NoError(t, err)
	assert.Equal(t, userID, gotUserID)
}

func TestSessionService_InvalidateUserSessions_RemovesAllTokens(t *testing.T) {
	repositories := newTestRepositories()
	userService := NewUserService(repositories.user, newTestPasswordHasher())
	_, err := userService.SignUp("alice", "pw")
	require.NoError(t, err)

	cache := cacheInMemory.NewInMemoryCache()
	svc := NewSessionService(userService, auth.NewJwtTokenProvider("test-secret"), cache)

	token1, err := svc.Login("alice", "pw")
	require.NoError(t, err)
	token2, err := svc.Login("alice", "pw")
	require.NoError(t, err)

	userID, err := userService.VerifyCredentials("alice", "pw")
	require.NoError(t, err)
	require.NoError(t, svc.InvalidateUserSessions(userID))

	_, err = svc.ValidateTokenToId(token1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidToken))

	_, err = svc.ValidateTokenToId(token2)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidToken))
}
