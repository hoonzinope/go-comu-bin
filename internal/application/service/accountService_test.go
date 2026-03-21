package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	cacheInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failOnTokenDeleteCache struct {
	mu          sync.Mutex
	data        map[string]struct{}
	deleteCalls int
}

func (c *failOnTokenDeleteCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.data[key]
	return nil, ok, nil
}

func (c *failOnTokenDeleteCache) Set(ctx context.Context, key string, value interface{}) error {
	_ = ctx
	_ = value
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data == nil {
		c.data = map[string]struct{}{}
	}
	c.data[key] = struct{}{}
	return nil
}

func (c *failOnTokenDeleteCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttlSeconds int) error {
	_ = ttlSeconds
	return c.Set(ctx, key, value)
}

func (c *failOnTokenDeleteCache) Delete(ctx context.Context, key string) error {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteCalls++
	if c.deleteCalls == 1 {
		return errors.New("delete current session failed")
	}
	delete(c.data, key)
	return nil
}

func (c *failOnTokenDeleteCache) DeleteByPrefix(ctx context.Context, prefix string) (int, error) {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	deleted := 0
	for key := range c.data {
		if strings.HasPrefix(key, prefix) {
			delete(c.data, key)
			deleted++
		}
	}
	return deleted, nil
}

func (c *failOnTokenDeleteCache) ExistsByPrefix(ctx context.Context, prefix string) (bool, error) {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.data {
		if strings.HasPrefix(key, prefix) {
			return true, nil
		}
	}
	return false, nil
}

func (c *failOnTokenDeleteCache) GetOrSetWithTTL(ctx context.Context, key string, ttlSeconds int, loader func(context.Context) (interface{}, error)) (interface{}, error) {
	_ = key
	_ = ttlSeconds
	return loader(ctx)
}

func (c *failOnTokenDeleteCache) activeNewTokens() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0)
	for key := range c.data {
		out = append(out, key)
	}
	return out
}

type stubUserUseCase struct {
	deleteMe func(ctx context.Context, userID int64, password string) error
}

func (s *stubUserUseCase) SignUp(ctx context.Context, username, password string) (string, error) {
	return "ok", nil
}

func (s *stubUserUseCase) IssueGuestAccount(ctx context.Context) (int64, error) {
	return 0, nil
}

func (s *stubUserUseCase) UpgradeGuest(ctx context.Context, userID int64, username, email, password string) error {
	return nil
}

func (s *stubUserUseCase) DeleteMe(ctx context.Context, userID int64, password string) error {
	if s.deleteMe != nil {
		return s.deleteMe(ctx, userID, password)
	}
	return nil
}

func (s *stubUserUseCase) GetUserSuspension(ctx context.Context, adminID int64, targetUserUUID string) (*model.UserSuspension, error) {
	return &model.UserSuspension{
		UserUUID:       targetUserUUID,
		Status:         entity.UserStatusActive,
		SuspendedUntil: nil,
	}, nil
}

func (s *stubUserUseCase) SuspendUser(ctx context.Context, adminID int64, targetUserUUID, reason string, duration model.SuspensionDuration) error {
	return nil
}

func (s *stubUserUseCase) UnsuspendUser(ctx context.Context, adminID int64, targetUserUUID string) error {
	return nil
}

type stubSessionUseCase struct {
	invalidateUserSessions func(ctx context.Context, userID int64) error
}

func (s *stubSessionUseCase) Login(ctx context.Context, username, password string) (string, error) {
	return "", nil
}

func (s *stubSessionUseCase) IssueGuestToken(ctx context.Context) (string, error) {
	return "", nil
}

func (s *stubSessionUseCase) RotateToken(ctx context.Context, userID int64, currentToken string) (string, error) {
	return "", nil
}

func (s *stubSessionUseCase) Logout(ctx context.Context, token string) error {
	return nil
}

func (s *stubSessionUseCase) InvalidateUserSessions(ctx context.Context, userID int64) error {
	if s.invalidateUserSessions != nil {
		return s.invalidateUserSessions(ctx, userID)
	}
	return nil
}

func (s *stubSessionUseCase) ValidateTokenToId(ctx context.Context, token string) (int64, error) {
	return 0, nil
}

func TestAccountService_DeleteMyAccount_Success(t *testing.T) {
	calledDeleteMe := false
	calledInvalidate := false
	svc := NewAccountService(
		&stubUserUseCase{
			deleteMe: func(ctx context.Context, userID int64, password string) error {
				calledDeleteMe = true
				assert.Equal(t, int64(10), userID)
				assert.Equal(t, "pw", password)
				return nil
			},
		},
		&stubSessionUseCase{
			invalidateUserSessions: func(ctx context.Context, userID int64) error {
				calledInvalidate = true
				assert.Equal(t, int64(10), userID)
				return nil
			},
		},
	)

	require.NoError(t, svc.DeleteMyAccount(context.Background(), 10, "pw"))
	assert.True(t, calledDeleteMe)
	assert.True(t, calledInvalidate)
}

func TestAccountService_DeleteMyAccount_StopsOnDeleteFailure(t *testing.T) {
	calledInvalidate := false
	svc := NewAccountService(
		&stubUserUseCase{
			deleteMe: func(ctx context.Context, userID int64, password string) error {
				return customerror.ErrInvalidCredential
			},
		},
		&stubSessionUseCase{
			invalidateUserSessions: func(ctx context.Context, userID int64) error {
				calledInvalidate = true
				return nil
			},
		},
	)

	err := svc.DeleteMyAccount(context.Background(), 10, "bad")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidCredential))
	assert.False(t, calledInvalidate)
}

func TestAccountService_DeleteMyAccount_IgnoresSessionInvalidationFailure(t *testing.T) {
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	calledInvalidate := false
	svc := NewAccountService(
		&stubUserUseCase{
			deleteMe: func(ctx context.Context, userID int64, password string) error {
				return nil
			},
		},
		&stubSessionUseCase{
			invalidateUserSessions: func(ctx context.Context, userID int64) error {
				calledInvalidate = true
				return customerror.WrapRepository("delete sessions", errors.New("cache unavailable"))
			},
		},
	)

	err := svc.DeleteMyAccount(context.Background(), 10, "pw")
	require.NoError(t, err)
	assert.True(t, calledInvalidate)
}

func TestAccountService_UpgradeGuestAccount_Success(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "hashed-secret")
	guestID, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)

	cache := auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache())
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	oldToken, err := tokenProvider.IdToToken(guestID)
	require.NoError(t, err)
	require.NoError(t, cache.Save(context.Background(), guestID, oldToken, tokenProvider.TTLSeconds()))

	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		tokenProvider,
		cache,
	)

	newToken, err := svc.UpgradeGuestAccount(context.Background(), guestID, oldToken, "alice", "alice@example.com", "pw")
	require.NoError(t, err)
	assert.NotEmpty(t, newToken)
	assert.NotEqual(t, oldToken, newToken)

	upgraded, err := repositories.user.SelectUserByID(context.Background(), guestID)
	require.NoError(t, err)
	require.NotNil(t, upgraded)
	assert.False(t, upgraded.IsGuest())
	assert.Equal(t, "alice", upgraded.Name)
	assert.Equal(t, "alice@example.com", upgraded.Email)

	oldExists, err := cache.Exists(context.Background(), guestID, oldToken)
	require.NoError(t, err)
	assert.False(t, oldExists)

	newExists, err := cache.Exists(context.Background(), guestID, newToken)
	require.NoError(t, err)
	assert.True(t, newExists)
}

func TestAccountService_UpgradeGuestAccount_RollsBackWhenSessionDeleteFails(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "hashed-secret")
	guestID, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)

	cacheBackend := &failOnTokenDeleteCache{}
	sessionRepository := auth.NewCacheSessionRepository(cacheBackend)
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	oldToken, err := tokenProvider.IdToToken(guestID)
	require.NoError(t, err)
	require.NoError(t, sessionRepository.Save(context.Background(), guestID, oldToken, tokenProvider.TTLSeconds()))

	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		tokenProvider,
		sessionRepository,
	)

	_, err = svc.UpgradeGuestAccount(context.Background(), guestID, oldToken, "alice", "alice@example.com", "pw")
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrRepositoryFailure))

	userAfter, err := repositories.user.SelectUserByID(context.Background(), guestID)
	require.NoError(t, err)
	require.NotNil(t, userAfter)
	assert.True(t, userAfter.IsGuest())
	assert.Equal(t, "guest-1", userAfter.Name)

	oldExists, err := sessionRepository.Exists(context.Background(), guestID, oldToken)
	require.NoError(t, err)
	assert.True(t, oldExists)

	remainingTokens := cacheBackend.activeNewTokens()
	require.Len(t, remainingTokens, 1)
	assert.Contains(t, remainingTokens[0], oldToken)
}
