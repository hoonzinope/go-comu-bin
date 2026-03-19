package service

import (
	"context"
	"errors"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	cacheInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
)

type recordingCredentialVerifier struct {
	calledCtx context.Context
	userID    int64
	err       error
}

func (v *recordingCredentialVerifier) VerifyCredentials(ctx context.Context, username, password string) (int64, error) {
	v.calledCtx = ctx
	if v.err != nil {
		return 0, v.err
	}
	return v.userID, nil
}

func TestSessionService_Login_Success(t *testing.T) {
	repositories := newTestRepositories()
	userService := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "pw")
	require.NoError(t, err)

	cache := cacheInMemory.NewInMemoryCache()
	sessionRepository := auth.NewCacheSessionRepository(cache)
	svc := NewSessionService(userService, userService, repositories.user, auth.NewJwtTokenProvider("test-secret"), sessionRepository)

	token, err := svc.Login(context.Background(), "alice", "pw")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	user, err := repositories.user.SelectUserByUsername(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, user)
	exists, err := sessionRepository.Exists(context.Background(), user.ID, token)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestSessionService_IssueGuestToken_Success(t *testing.T) {
	repositories := newTestRepositories()
	userService := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)

	cache := cacheInMemory.NewInMemoryCache()
	sessionRepository := auth.NewCacheSessionRepository(cache)
	svc := NewSessionService(userService, userService, repositories.user, auth.NewJwtTokenProvider("test-secret"), sessionRepository)

	token, err := svc.IssueGuestToken(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	userID, err := svc.ValidateTokenToId(context.Background(), token)
	require.NoError(t, err)
	user, err := repositories.user.SelectUserByID(context.Background(), userID)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.True(t, user.IsGuest())
}

func TestSessionService_Login_PropagatesContextToCredentialVerifier(t *testing.T) {
	repositories := newTestRepositories()
	cache := cacheInMemory.NewInMemoryCache()
	sessionRepository := auth.NewCacheSessionRepository(cache)
	verifier := &recordingCredentialVerifier{userID: 42}
	svc := NewSessionService(verifier, nil, repositories.user, auth.NewJwtTokenProvider("test-secret"), sessionRepository)

	ctx := context.WithValue(context.Background(), struct{ key string }{key: "req"}, "v")
	_, err := svc.Login(ctx, "alice", "pw")
	require.NoError(t, err)
	assert.Same(t, ctx, verifier.calledCtx)
}

func TestSessionService_ValidateTokenToId_InvalidatedToken(t *testing.T) {
	repositories := newTestRepositories()
	userService := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "pw")
	require.NoError(t, err)

	cache := cacheInMemory.NewInMemoryCache()
	sessionRepository := auth.NewCacheSessionRepository(cache)
	svc := NewSessionService(userService, userService, repositories.user, auth.NewJwtTokenProvider("test-secret"), sessionRepository)

	token, err := svc.Login(context.Background(), "alice", "pw")
	require.NoError(t, err)
	require.NoError(t, svc.Logout(context.Background(), token))

	_, err = svc.ValidateTokenToId(context.Background(), token)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidToken))
}

func TestSessionService_ValidateTokenToId_Success(t *testing.T) {
	repositories := newTestRepositories()
	userID := seedUser(repositories.user, "alice", "pw", "user")
	userService := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)

	cache := cacheInMemory.NewInMemoryCache()
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(userID)
	require.NoError(t, err)
	sessionRepository := auth.NewCacheSessionRepository(cache)
	require.NoError(t, sessionRepository.Save(context.Background(), userID, token, tokenProvider.TTLSeconds()))

	svc := NewSessionService(userService, userService, repositories.user, tokenProvider, sessionRepository)

	gotUserID, err := svc.ValidateTokenToId(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, userID, gotUserID)
}

func TestSessionService_ValidateTokenToId_DeletedUser(t *testing.T) {
	repositories := newTestRepositories()
	userService := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "pw")
	require.NoError(t, err)
	userID, err := userService.VerifyCredentials(context.Background(), "alice", "pw")
	require.NoError(t, err)

	cache := cacheInMemory.NewInMemoryCache()
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(userID)
	require.NoError(t, err)
	sessionRepository := auth.NewCacheSessionRepository(cache)
	require.NoError(t, sessionRepository.Save(context.Background(), userID, token, tokenProvider.TTLSeconds()))

	require.NoError(t, userService.DeleteMe(context.Background(), userID, "pw"))

	svc := NewSessionService(userService, userService, repositories.user, tokenProvider, sessionRepository)

	_, err = svc.ValidateTokenToId(context.Background(), token)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidToken))
}

func TestSessionService_InvalidateUserSessions_RemovesAllTokens(t *testing.T) {
	repositories := newTestRepositories()
	userService := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "pw")
	require.NoError(t, err)

	cache := cacheInMemory.NewInMemoryCache()
	sessionRepository := auth.NewCacheSessionRepository(cache)
	svc := NewSessionService(userService, userService, repositories.user, auth.NewJwtTokenProvider("test-secret"), sessionRepository)

	token1, err := svc.Login(context.Background(), "alice", "pw")
	require.NoError(t, err)
	token2, err := svc.Login(context.Background(), "alice", "pw")
	require.NoError(t, err)

	userID, err := userService.VerifyCredentials(context.Background(), "alice", "pw")
	require.NoError(t, err)
	require.NoError(t, svc.InvalidateUserSessions(context.Background(), userID))

	_, err = svc.ValidateTokenToId(context.Background(), token1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidToken))

	_, err = svc.ValidateTokenToId(context.Background(), token2)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidToken))
}

func TestSessionService_Login_ReturnsRepositoryFailure_WhenSessionStoreSaveFails(t *testing.T) {
	repositories := newTestRepositories()
	userService := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "pw")
	require.NoError(t, err)

	sessionRepository := auth.NewCacheSessionRepository(&errorCache{
		setWithTTLErr: newCacheFailure(nil),
	})
	svc := NewSessionService(userService, userService, repositories.user, auth.NewJwtTokenProvider("test-secret"), sessionRepository)

	_, err = svc.Login(context.Background(), "alice", "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrRepositoryFailure))
}

func TestSessionService_Logout_ReturnsRepositoryFailure_WhenSessionDeleteFails(t *testing.T) {
	repositories := newTestRepositories()
	userService := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "pw")
	require.NoError(t, err)

	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	userID, err := userService.VerifyCredentials(context.Background(), "alice", "pw")
	require.NoError(t, err)
	token, err := tokenProvider.IdToToken(userID)
	require.NoError(t, err)

	sessionRepository := auth.NewCacheSessionRepository(&errorCache{
		deleteErr: newCacheFailure(nil),
	})
	svc := NewSessionService(userService, userService, repositories.user, tokenProvider, sessionRepository)

	err = svc.Logout(context.Background(), token)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrRepositoryFailure))
}

func TestSessionService_InvalidateUserSessions_ReturnsRepositoryFailure_WhenSessionDeleteFails(t *testing.T) {
	repositories := newTestRepositories()
	userService := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "pw")
	require.NoError(t, err)

	userID, err := userService.VerifyCredentials(context.Background(), "alice", "pw")
	require.NoError(t, err)

	sessionRepository := auth.NewCacheSessionRepository(&errorCache{
		deleteByPrefixErr: newCacheFailure(nil),
	})
	svc := NewSessionService(userService, userService, repositories.user, auth.NewJwtTokenProvider("test-secret"), sessionRepository)

	err = svc.InvalidateUserSessions(context.Background(), userID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrRepositoryFailure))
}
