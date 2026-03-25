package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	cacheInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
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
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "pw")
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
	assert.Equal(t, entity.GuestStatusActive, user.GuestStatus)
	assert.NotNil(t, user.GuestActivatedAt)
}

func TestSessionService_RotateToken_ReplacesCurrentSession(t *testing.T) {
	repositories := newTestRepositories()
	userService := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.NoError(t, err)

	cache := cacheInMemory.NewInMemoryCache()
	sessionRepository := auth.NewCacheSessionRepository(cache)
	svc := NewSessionService(userService, userService, repositories.user, auth.NewJwtTokenProvider("test-secret"), sessionRepository)

	oldToken, err := svc.Login(context.Background(), "alice", "pw")
	require.NoError(t, err)
	userID, err := userService.VerifyCredentials(context.Background(), "alice", "pw")
	require.NoError(t, err)

	newToken, err := svc.RotateToken(context.Background(), userID, oldToken)
	require.NoError(t, err)
	assert.NotEmpty(t, newToken)
	assert.NotEqual(t, oldToken, newToken)

	_, err = svc.ValidateTokenToId(context.Background(), oldToken)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidToken))

	gotUserID, err := svc.ValidateTokenToId(context.Background(), newToken)
	require.NoError(t, err)
	assert.Equal(t, userID, gotUserID)
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

type recordingGuestIssuer struct {
	userRepo port.UserRepository
	userID   int64
}

type failOnNthUpdateUserRepository struct {
	base        port.UserRepository
	failOn      int
	updateCalls int
	err         error
}

func (r *failOnNthUpdateUserRepository) Save(ctx context.Context, user *entity.User) (int64, error) {
	return r.base.Save(ctx, user)
}

func (r *failOnNthUpdateUserRepository) SelectUserByUsername(ctx context.Context, username string) (*entity.User, error) {
	return r.base.SelectUserByUsername(ctx, username)
}

func (r *failOnNthUpdateUserRepository) SelectUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	return r.base.SelectUserByEmail(ctx, email)
}

func (r *failOnNthUpdateUserRepository) SelectUserByUUID(ctx context.Context, userUUID string) (*entity.User, error) {
	return r.base.SelectUserByUUID(ctx, userUUID)
}

func (r *failOnNthUpdateUserRepository) SelectUserByID(ctx context.Context, id int64) (*entity.User, error) {
	return r.base.SelectUserByID(ctx, id)
}

func (r *failOnNthUpdateUserRepository) SelectUserByIDIncludingDeleted(ctx context.Context, id int64) (*entity.User, error) {
	return r.base.SelectUserByIDIncludingDeleted(ctx, id)
}

func (r *failOnNthUpdateUserRepository) SelectUsersByIDsIncludingDeleted(ctx context.Context, ids []int64) (map[int64]*entity.User, error) {
	return r.base.SelectUsersByIDsIncludingDeleted(ctx, ids)
}

func (r *failOnNthUpdateUserRepository) SelectGuestCleanupCandidates(ctx context.Context, now time.Time, pendingGrace, activeUnusedGrace time.Duration, limit int) ([]*entity.User, error) {
	return r.base.SelectGuestCleanupCandidates(ctx, now, pendingGrace, activeUnusedGrace, limit)
}

func (r *failOnNthUpdateUserRepository) Update(ctx context.Context, user *entity.User) error {
	r.updateCalls++
	if r.failOn > 0 && r.updateCalls == r.failOn {
		if r.err != nil {
			return r.err
		}
		return errors.New("forced guest activation failure")
	}
	return r.base.Update(ctx, user)
}

func (r *failOnNthUpdateUserRepository) Delete(ctx context.Context, id int64) error {
	return r.base.Delete(ctx, id)
}

func (g *recordingGuestIssuer) IssueGuestAccount(ctx context.Context) (int64, error) {
	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "hashed-secret")
	userID, err := g.userRepo.Save(ctx, guest)
	if err != nil {
		return 0, err
	}
	g.userID = userID
	return userID, nil
}

func TestSessionService_IssueGuestToken_ExpiresGuestWhenSessionStoreSaveFails(t *testing.T) {
	repositories := newTestRepositories()
	guestIssuer := &recordingGuestIssuer{userRepo: repositories.user}
	sessionRepository := auth.NewCacheSessionRepository(&errorCache{
		setWithTTLErr: newCacheFailure(nil),
	})
	svc := NewSessionService(&recordingCredentialVerifier{}, guestIssuer, repositories.user, auth.NewJwtTokenProvider("test-secret"), sessionRepository)

	_, err := svc.IssueGuestToken(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrRepositoryFailure))

	user, err := repositories.user.SelectUserByID(context.Background(), guestIssuer.userID)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.True(t, user.IsGuest())
	assert.Equal(t, entity.GuestStatusExpired, user.GuestStatus)
	assert.NotNil(t, user.GuestExpiredAt)
}

func TestSessionService_IssueGuestToken_DoesNotPersistSessionWhenGuestActivationFails(t *testing.T) {
	repositories := newTestRepositories()
	wrappedUserRepo := &failOnNthUpdateUserRepository{
		base:   repositories.user,
		failOn: 1,
		err:    errors.New("activate guest failed"),
	}
	guestIssuer := &recordingGuestIssuer{userRepo: wrappedUserRepo}
	cache := cacheInMemory.NewInMemoryCache()
	sessionRepository := auth.NewCacheSessionRepository(cache)
	svc := NewSessionService(&recordingCredentialVerifier{}, guestIssuer, wrappedUserRepo, auth.NewJwtTokenProvider("test-secret"), sessionRepository)

	token, err := svc.IssueGuestToken(context.Background())
	require.Error(t, err)
	assert.Empty(t, token)
	assert.True(t, errors.Is(err, customerror.ErrRepositoryFailure))

	exists, err := sessionRepository.ExistsByUser(context.Background(), guestIssuer.userID)
	require.NoError(t, err)
	assert.False(t, exists)

	user, err := repositories.user.SelectUserByID(context.Background(), guestIssuer.userID)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.True(t, user.IsGuest())
	assert.NotEqual(t, entity.GuestStatusActive, user.GuestStatus)
}

func TestSessionService_ValidateTokenToId_InvalidatedToken(t *testing.T) {
	repositories := newTestRepositories()
	userService := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "pw")
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

func TestSessionService_ValidateTokenToId_RejectsPendingGuest(t *testing.T) {
	repositories := newTestRepositories()
	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "pw")
	userID, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)

	cache := cacheInMemory.NewInMemoryCache()
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(userID)
	require.NoError(t, err)
	sessionRepository := auth.NewCacheSessionRepository(cache)
	require.NoError(t, sessionRepository.Save(context.Background(), userID, token, tokenProvider.TTLSeconds()))

	svc := NewSessionService(&recordingCredentialVerifier{}, nil, repositories.user, tokenProvider, sessionRepository)

	_, err = svc.ValidateTokenToId(context.Background(), token)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidToken))
}

func TestSessionService_ValidateTokenToId_RejectsExpiredGuest(t *testing.T) {
	repositories := newTestRepositories()
	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "pw")
	guest.MarkGuestExpired()
	userID, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)

	cache := cacheInMemory.NewInMemoryCache()
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	token, err := tokenProvider.IdToToken(userID)
	require.NoError(t, err)
	sessionRepository := auth.NewCacheSessionRepository(cache)
	require.NoError(t, sessionRepository.Save(context.Background(), userID, token, tokenProvider.TTLSeconds()))

	svc := NewSessionService(&recordingCredentialVerifier{}, nil, repositories.user, tokenProvider, sessionRepository)

	_, err = svc.ValidateTokenToId(context.Background(), token)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidToken))
}

func TestSessionService_ValidateTokenToId_DeletedUser(t *testing.T) {
	repositories := newTestRepositories()
	userService := NewUserService(repositories.user, newTestPasswordHasher(), repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "pw")
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
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "pw")
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
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "pw")
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
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "pw")
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
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "pw")
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
