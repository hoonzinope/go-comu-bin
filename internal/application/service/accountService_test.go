package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
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

type recordingPasswordResetMailSender struct {
	sent []struct {
		email string
		token string
	}
	err error
}

type recordingEmailVerificationMailSender struct {
	sent []struct {
		email string
		token string
	}
	err error
}

func newRecordingPasswordResetMailSender() *recordingPasswordResetMailSender {
	return &recordingPasswordResetMailSender{}
}

func (s *recordingPasswordResetMailSender) SendPasswordReset(ctx context.Context, email, token string, expiresAt time.Time) error {
	_ = ctx
	_ = expiresAt
	s.sent = append(s.sent, struct {
		email string
		token string
	}{email: email, token: token})
	return s.err
}

func newRecordingEmailVerificationMailSender() *recordingEmailVerificationMailSender {
	return &recordingEmailVerificationMailSender{}
}

func (s *recordingEmailVerificationMailSender) SendEmailVerification(ctx context.Context, email, token string, expiresAt time.Time) error {
	_ = ctx
	_ = expiresAt
	s.sent = append(s.sent, struct {
		email string
		token string
	}{email: email, token: token})
	return s.err
}

func testHashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func testHashEmailVerificationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type fixedPasswordResetTokenIssuer struct {
	tokens []string
	idx    int
	err    error
}

type fixedEmailVerificationTokenIssuer struct {
	tokens []string
	idx    int
	err    error
}

type failOnNthUserUpdateRepository struct {
	base        port.UserRepository
	failOn      int
	updateCalls int
	err         error
}

func (r *failOnNthUserUpdateRepository) Save(ctx context.Context, user *entity.User) (int64, error) {
	return r.base.Save(ctx, user)
}

func (r *failOnNthUserUpdateRepository) SelectUserByUsername(ctx context.Context, username string) (*entity.User, error) {
	return r.base.SelectUserByUsername(ctx, username)
}

func (r *failOnNthUserUpdateRepository) SelectUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	return r.base.SelectUserByEmail(ctx, email)
}

func (r *failOnNthUserUpdateRepository) SelectUserByUUID(ctx context.Context, userUUID string) (*entity.User, error) {
	return r.base.SelectUserByUUID(ctx, userUUID)
}

func (r *failOnNthUserUpdateRepository) SelectUserByID(ctx context.Context, id int64) (*entity.User, error) {
	return r.base.SelectUserByID(ctx, id)
}

func (r *failOnNthUserUpdateRepository) SelectUserByIDIncludingDeleted(ctx context.Context, id int64) (*entity.User, error) {
	return r.base.SelectUserByIDIncludingDeleted(ctx, id)
}

func (r *failOnNthUserUpdateRepository) SelectUsersByIDsIncludingDeleted(ctx context.Context, ids []int64) (map[int64]*entity.User, error) {
	return r.base.SelectUsersByIDsIncludingDeleted(ctx, ids)
}

func (r *failOnNthUserUpdateRepository) SelectGuestCleanupCandidates(ctx context.Context, now time.Time, pendingGrace, activeUnusedGrace time.Duration, limit int) ([]*entity.User, error) {
	return r.base.SelectGuestCleanupCandidates(ctx, now, pendingGrace, activeUnusedGrace, limit)
}

func (r *failOnNthUserUpdateRepository) Update(ctx context.Context, user *entity.User) error {
	r.updateCalls++
	if r.failOn > 0 && r.updateCalls == r.failOn {
		if r.err != nil {
			return r.err
		}
		return errors.New("forced user update failure")
	}
	return r.base.Update(ctx, user)
}

func (r *failOnNthUserUpdateRepository) Delete(ctx context.Context, id int64) error {
	return r.base.Delete(ctx, id)
}

type accountTestTxScope struct {
	ctx           context.Context
	user          port.UserRepository
	passwordReset port.PasswordResetTokenRepository
}

func (s accountTestTxScope) Context() context.Context { return s.ctx }
func (s accountTestTxScope) UserRepository() port.UserRepository {
	return s.user
}
func (s accountTestTxScope) BoardRepository() port.BoardRepository { return nil }
func (s accountTestTxScope) PostRepository() port.PostRepository   { return nil }
func (s accountTestTxScope) TagRepository() port.TagRepository     { return nil }
func (s accountTestTxScope) PostTagRepository() port.PostTagRepository {
	return nil
}
func (s accountTestTxScope) CommentRepository() port.CommentRepository { return nil }
func (s accountTestTxScope) ReactionRepository() port.ReactionRepository {
	return nil
}
func (s accountTestTxScope) AttachmentRepository() port.AttachmentRepository { return nil }
func (s accountTestTxScope) ReportRepository() port.ReportRepository         { return nil }
func (s accountTestTxScope) NotificationRepository() port.NotificationRepository {
	return nil
}
func (s accountTestTxScope) EmailVerificationTokenRepository() port.EmailVerificationTokenRepository {
	return nil
}
func (s accountTestTxScope) PasswordResetTokenRepository() port.PasswordResetTokenRepository {
	return s.passwordReset
}
func (s accountTestTxScope) Outbox() port.OutboxAppender { return nil }

type accountTestUnitOfWork struct {
	scope accountTestTxScope
}

func (u accountTestUnitOfWork) WithinTransaction(ctx context.Context, fn func(tx port.TxScope) error) error {
	scope := u.scope
	scope.ctx = ctx
	return fn(scope)
}

func (i *fixedEmailVerificationTokenIssuer) Issue() (string, error) {
	if i.err != nil {
		return "", i.err
	}
	if i.idx >= len(i.tokens) {
		return "fixed-verification-token", nil
	}
	token := i.tokens[i.idx]
	i.idx++
	return token, nil
}

func (i *fixedPasswordResetTokenIssuer) Issue() (string, error) {
	if i.err != nil {
		return "", i.err
	}
	if i.idx >= len(i.tokens) {
		return "fixed-reset-token", nil
	}
	token := i.tokens[i.idx]
	i.idx++
	return token, nil
}

type failOnDeleteByPrefixCache struct {
	data map[string]struct{}
}

func (c *failOnDeleteByPrefixCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	_ = ctx
	_, ok := c.data[key]
	return nil, ok, nil
}

func (c *failOnDeleteByPrefixCache) Set(ctx context.Context, key string, value interface{}) error {
	_ = ctx
	_ = value
	if c.data == nil {
		c.data = map[string]struct{}{}
	}
	c.data[key] = struct{}{}
	return nil
}

func (c *failOnDeleteByPrefixCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttlSeconds int) error {
	_ = ttlSeconds
	return c.Set(ctx, key, value)
}

func (c *failOnDeleteByPrefixCache) DeleteByPrefix(ctx context.Context, prefix string) (int, error) {
	_ = ctx
	_ = prefix
	return 0, errors.New("delete by prefix failed")
}

func (c *failOnDeleteByPrefixCache) Delete(ctx context.Context, key string) error {
	_ = ctx
	delete(c.data, key)
	return nil
}

func (c *failOnDeleteByPrefixCache) ExistsByPrefix(ctx context.Context, prefix string) (bool, error) {
	_ = ctx
	for key := range c.data {
		if strings.HasPrefix(key, prefix) {
			return true, nil
		}
	}
	return false, nil
}

func (c *failOnDeleteByPrefixCache) GetOrSetWithTTL(ctx context.Context, key string, ttlSeconds int, loader func(context.Context) (interface{}, error)) (interface{}, error) {
	_ = key
	_ = ttlSeconds
	return loader(ctx)
}

type failOnNthSetWithTTLCache struct {
	mu              sync.Mutex
	data            map[string]struct{}
	failOn          int
	setWithTTLCalls int
}

func (c *failOnNthSetWithTTLCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.data[key]
	return nil, ok, nil
}

func (c *failOnNthSetWithTTLCache) Set(ctx context.Context, key string, value interface{}) error {
	return c.SetWithTTL(ctx, key, value, 0)
}

func (c *failOnNthSetWithTTLCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttlSeconds int) error {
	_ = ctx
	_ = value
	_ = ttlSeconds
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setWithTTLCalls++
	if c.failOn > 0 && c.setWithTTLCalls == c.failOn {
		return errors.New("session save failed")
	}
	if c.data == nil {
		c.data = map[string]struct{}{}
	}
	c.data[key] = struct{}{}
	return nil
}

func (c *failOnNthSetWithTTLCache) Delete(ctx context.Context, key string) error {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	return nil
}

func (c *failOnNthSetWithTTLCache) DeleteByPrefix(ctx context.Context, prefix string) (int, error) {
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

func (c *failOnNthSetWithTTLCache) ExistsByPrefix(ctx context.Context, prefix string) (bool, error) {
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

func (c *failOnNthSetWithTTLCache) GetOrSetWithTTL(ctx context.Context, key string, ttlSeconds int, loader func(context.Context) (interface{}, error)) (interface{}, error) {
	_ = key
	_ = ttlSeconds
	return loader(ctx)
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

func (s *stubUserUseCase) SignUp(ctx context.Context, username, email, password string) (string, error) {
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
		nil,
		nil,
		nil,
		0,
		repositories.passwordReset,
		auth.NewPasswordResetTokenIssuer(),
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
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
		nil,
		nil,
		nil,
		0,
		repositories.passwordReset,
		auth.NewPasswordResetTokenIssuer(),
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
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

func TestAccountService_UpgradeGuestAccount_LogsSucceeded(t *testing.T) {
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

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		tokenProvider,
		cache,
		nil,
		nil,
		nil,
		0,
		repositories.passwordReset,
		auth.NewPasswordResetTokenIssuer(),
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
		logger,
	)

	_, err = svc.UpgradeGuestAccount(context.Background(), guestID, oldToken, "alice", "alice@example.com", "pw")
	require.NoError(t, err)
	assert.Contains(t, logBuf.String(), `"event":"guest_upgrade_attempt"`)
	assert.Contains(t, logBuf.String(), `"outcome":"succeeded"`)
	assert.Contains(t, logBuf.String(), `"user_id":1`)
	assert.NotContains(t, logBuf.String(), "alice@example.com")
}

func TestAccountService_UpgradeGuestAccount_LogsInvalidToken(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "hashed-secret")
	guestID, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)

	cache := auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache())
	tokenProvider := auth.NewJwtTokenProvider("test-secret")

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		tokenProvider,
		cache,
		nil,
		nil,
		nil,
		0,
		repositories.passwordReset,
		auth.NewPasswordResetTokenIssuer(),
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
		logger,
	)

	_, err = svc.UpgradeGuestAccount(context.Background(), guestID, "missing-token", "alice", "alice@example.com", "pw")
	require.Error(t, err)
	assert.ErrorIs(t, err, customerror.ErrInvalidToken)
	assert.Contains(t, logBuf.String(), `"event":"guest_upgrade_attempt"`)
	assert.Contains(t, logBuf.String(), `"outcome":"invalid_token"`)
}

func TestAccountService_RequestPasswordReset_CreatesTokenAndSendsMail(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.NoError(t, err)

	mailer := newRecordingPasswordResetMailSender()
	issuer := &fixedPasswordResetTokenIssuer{tokens: []string{"reset-token-1"}}
	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		auth.NewJwtTokenProvider("test-secret"),
		auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache()),
		nil,
		nil,
		nil,
		0,
		repositories.passwordReset,
		issuer,
		mailer,
		30*time.Minute,
	)

	require.NoError(t, svc.RequestPasswordReset(context.Background(), "alice@example.com"))
	require.Len(t, mailer.sent, 1)
	assert.Equal(t, "alice@example.com", mailer.sent[0].email)
	assert.Equal(t, "reset-token-1", mailer.sent[0].token)

	saved, err := repositories.passwordReset.SelectByTokenHash(context.Background(), testHashResetToken("reset-token-1"))
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.True(t, saved.IsUsable(time.Now()))
}

func TestAccountService_RequestEmailVerification_CreatesTokenAndSendsMail(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.NoError(t, err)
	user, err := repositories.user.SelectUserByUsername(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	mailer := newRecordingEmailVerificationMailSender()
	issuer := &fixedEmailVerificationTokenIssuer{tokens: []string{"verify-token-1"}}
	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		auth.NewJwtTokenProvider("test-secret"),
		auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache()),
		repositories.emailVerification,
		issuer,
		mailer,
		30*time.Minute,
		repositories.passwordReset,
		&fixedPasswordResetTokenIssuer{},
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
	)

	require.NoError(t, svc.RequestEmailVerification(context.Background(), user.ID))
	require.Len(t, mailer.sent, 1)
	assert.Equal(t, "alice@example.com", mailer.sent[0].email)
	assert.Equal(t, "verify-token-1", mailer.sent[0].token)

	saved, err := repositories.emailVerification.SelectByTokenHash(context.Background(), testHashEmailVerificationToken("verify-token-1"))
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.True(t, saved.IsUsable(time.Now()))
}

func TestAccountService_ConfirmEmailVerification_VerifiesUserAndConsumesTokens(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.NoError(t, err)
	user, err := repositories.user.SelectUserByUsername(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	verificationToken := entity.NewEmailVerificationToken(user.ID, testHashEmailVerificationToken("verify-token-1"), time.Now().Add(time.Hour))
	require.NoError(t, repositories.emailVerification.Save(context.Background(), verificationToken))

	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		auth.NewJwtTokenProvider("test-secret"),
		auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache()),
		repositories.emailVerification,
		&fixedEmailVerificationTokenIssuer{},
		newRecordingEmailVerificationMailSender(),
		30*time.Minute,
		repositories.passwordReset,
		&fixedPasswordResetTokenIssuer{},
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
	)

	require.NoError(t, svc.ConfirmEmailVerification(context.Background(), "verify-token-1"))

	updatedUser, err := repositories.user.SelectUserByUsername(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, updatedUser)
	assert.True(t, updatedUser.IsEmailVerified())

	savedToken, err := repositories.emailVerification.SelectByTokenHash(context.Background(), testHashEmailVerificationToken("verify-token-1"))
	require.NoError(t, err)
	require.NotNil(t, savedToken)
	assert.True(t, savedToken.IsConsumed())
}

func TestAccountService_RequestPasswordReset_InvalidatesPreviousToken(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.NoError(t, err)

	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		auth.NewJwtTokenProvider("test-secret"),
		auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache()),
		nil,
		nil,
		nil,
		0,
		repositories.passwordReset,
		&fixedPasswordResetTokenIssuer{tokens: []string{"reset-token-1", "reset-token-2"}},
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
	)

	require.NoError(t, svc.RequestPasswordReset(context.Background(), "alice@example.com"))
	require.NoError(t, svc.RequestPasswordReset(context.Background(), "alice@example.com"))

	first, err := repositories.passwordReset.SelectByTokenHash(context.Background(), testHashResetToken("reset-token-1"))
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.True(t, first.IsConsumed())

	second, err := repositories.passwordReset.SelectByTokenHash(context.Background(), testHashResetToken("reset-token-2"))
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.True(t, second.IsUsable(time.Now()))
}

func TestAccountService_RequestPasswordReset_IgnoresUnknownEmail(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	mailer := newRecordingPasswordResetMailSender()
	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		auth.NewJwtTokenProvider("test-secret"),
		auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache()),
		nil,
		nil,
		nil,
		0,
		repositories.passwordReset,
		&fixedPasswordResetTokenIssuer{tokens: []string{"reset-token-1"}},
		mailer,
		30*time.Minute,
	)

	require.NoError(t, svc.RequestPasswordReset(context.Background(), "missing@example.com"))
	assert.Empty(t, mailer.sent)
}

func TestAccountService_ConfirmPasswordReset_UpdatesPasswordConsumesTokenAndInvalidatesSessions(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "oldpw")
	require.NoError(t, err)
	user, err := repositories.user.SelectUserByUsername(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	resetToken := entity.NewPasswordResetToken(user.ID, testHashResetToken("reset-token-1"), time.Now().Add(time.Hour))
	require.NoError(t, repositories.passwordReset.Save(context.Background(), resetToken))

	sessionRepository := auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache())
	require.NoError(t, sessionRepository.Save(context.Background(), user.ID, "token-a", 60))
	require.NoError(t, sessionRepository.Save(context.Background(), user.ID, "token-b", 60))

	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		auth.NewJwtTokenProvider("test-secret"),
		sessionRepository,
		nil,
		nil,
		nil,
		0,
		repositories.passwordReset,
		&fixedPasswordResetTokenIssuer{},
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
	)

	require.NoError(t, svc.ConfirmPasswordReset(context.Background(), "reset-token-1", "newpw"))

	_, err = userService.VerifyCredentials(context.Background(), "alice", "oldpw")
	require.Error(t, err)
	assert.ErrorIs(t, err, customerror.ErrInvalidCredential)

	userID, err := userService.VerifyCredentials(context.Background(), "alice", "newpw")
	require.NoError(t, err)
	assert.Equal(t, user.ID, userID)

	savedToken, err := repositories.passwordReset.SelectByTokenHash(context.Background(), testHashResetToken("reset-token-1"))
	require.NoError(t, err)
	require.NotNil(t, savedToken)
	assert.True(t, savedToken.IsConsumed())

	exists, err := sessionRepository.ExistsByUser(context.Background(), user.ID)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestAccountService_ConfirmPasswordReset_RollsBackWhenSessionInvalidationFails(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "oldpw")
	require.NoError(t, err)
	user, err := repositories.user.SelectUserByUsername(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	resetToken := entity.NewPasswordResetToken(user.ID, testHashResetToken("reset-token-1"), time.Now().Add(time.Hour))
	require.NoError(t, repositories.passwordReset.Save(context.Background(), resetToken))

	sessionRepository := auth.NewCacheSessionRepository(&failOnDeleteByPrefixCache{})
	require.NoError(t, sessionRepository.Save(context.Background(), user.ID, "token-a", 60))

	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		auth.NewJwtTokenProvider("test-secret"),
		sessionRepository,
		nil,
		nil,
		nil,
		0,
		repositories.passwordReset,
		&fixedPasswordResetTokenIssuer{},
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
	)

	err = svc.ConfirmPasswordReset(context.Background(), "reset-token-1", "newpw")
	require.Error(t, err)
	assert.ErrorIs(t, err, customerror.ErrRepositoryFailure)

	userID, err := userService.VerifyCredentials(context.Background(), "alice", "oldpw")
	require.NoError(t, err)
	assert.Equal(t, user.ID, userID)

	savedToken, err := repositories.passwordReset.SelectByTokenHash(context.Background(), testHashResetToken("reset-token-1"))
	require.NoError(t, err)
	require.NotNil(t, savedToken)
	assert.False(t, savedToken.IsConsumed())
}

func TestAccountService_ConfirmPasswordReset_RestoresStateAfterSessionInvalidationFails(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "oldpw")
	require.NoError(t, err)
	user, err := repositories.user.SelectUserByUsername(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	resetToken := entity.NewPasswordResetToken(user.ID, testHashResetToken("reset-token-1"), time.Now().Add(time.Hour))
	require.NoError(t, repositories.passwordReset.Save(context.Background(), resetToken))

	sessionRepository := auth.NewCacheSessionRepository(&failOnDeleteByPrefixCache{})
	require.NoError(t, sessionRepository.Save(context.Background(), user.ID, "token-a", 60))

	wrappedUserRepo := &failOnNthUserUpdateRepository{
		base: repositories.user,
	}
	unitOfWork := accountTestUnitOfWork{
		scope: accountTestTxScope{
			user:          wrappedUserRepo,
			passwordReset: repositories.passwordReset,
		},
	}

	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		wrappedUserRepo,
		unitOfWork,
		passwordHasher,
		auth.NewJwtTokenProvider("test-secret"),
		sessionRepository,
		nil,
		nil,
		nil,
		0,
		repositories.passwordReset,
		&fixedPasswordResetTokenIssuer{},
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
	)

	err = svc.ConfirmPasswordReset(context.Background(), "reset-token-1", "newpw")
	require.Error(t, err)
	assert.ErrorIs(t, err, customerror.ErrRepositoryFailure)
	assert.Equal(t, 2, wrappedUserRepo.updateCalls)

	userID, err := userService.VerifyCredentials(context.Background(), "alice", "oldpw")
	require.NoError(t, err)
	assert.Equal(t, user.ID, userID)

	savedToken, err := repositories.passwordReset.SelectByTokenHash(context.Background(), testHashResetToken("reset-token-1"))
	require.NoError(t, err)
	require.NotNil(t, savedToken)
	assert.False(t, savedToken.IsConsumed())
}

func TestAccountService_UpgradeGuestAccount_RollsBackWhenNewSessionSaveFails(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	guest := entity.NewGuest("guest-1", "guest-1@example.invalid", "hashed-secret")
	guestID, err := repositories.user.Save(context.Background(), guest)
	require.NoError(t, err)

	cacheBackend := &failOnNthSetWithTTLCache{failOn: 2}
	sessionRepository := auth.NewCacheSessionRepository(cacheBackend)
	tokenProvider := auth.NewJwtTokenProvider("test-secret")
	oldToken, err := tokenProvider.IdToToken(guestID)
	require.NoError(t, err)
	require.NoError(t, sessionRepository.Save(context.Background(), guestID, oldToken, tokenProvider.TTLSeconds()))

	emailIssuer := &fixedEmailVerificationTokenIssuer{tokens: []string{"verify-token-1"}}
	mailer := newRecordingEmailVerificationMailSender()
	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		tokenProvider,
		sessionRepository,
		repositories.emailVerification,
		emailIssuer,
		mailer,
		30*time.Minute,
		repositories.passwordReset,
		&fixedPasswordResetTokenIssuer{},
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
	)

	_, err = svc.UpgradeGuestAccount(context.Background(), guestID, oldToken, "alice", "alice@example.com", "pw")
	require.Error(t, err)
	assert.ErrorIs(t, err, customerror.ErrRepositoryFailure)

	userAfter, err := repositories.user.SelectUserByID(context.Background(), guestID)
	require.NoError(t, err)
	require.NotNil(t, userAfter)
	assert.True(t, userAfter.IsGuest())
	assert.Equal(t, "guest-1", userAfter.Name)

	oldExists, err := sessionRepository.Exists(context.Background(), guestID, oldToken)
	require.NoError(t, err)
	assert.True(t, oldExists)

	savedVerification, err := repositories.emailVerification.SelectByTokenHash(context.Background(), testHashEmailVerificationToken("verify-token-1"))
	require.NoError(t, err)
	require.NotNil(t, savedVerification)
	assert.True(t, savedVerification.IsConsumed())

	assert.Len(t, mailer.sent, 1)
}

func TestAccountService_RequestPasswordReset_LogsAuditOutcome(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.NoError(t, err)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		auth.NewJwtTokenProvider("test-secret"),
		auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache()),
		nil,
		nil,
		nil,
		0,
		repositories.passwordReset,
		&fixedPasswordResetTokenIssuer{tokens: []string{"reset-token-1"}},
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
		logger,
	)

	require.NoError(t, svc.RequestPasswordReset(context.Background(), "alice@example.com"))
	assert.Contains(t, logBuf.String(), `"event":"password_reset_request"`)
	assert.Contains(t, logBuf.String(), `"outcome":"issued"`)
	assert.NotContains(t, logBuf.String(), "alice@example.com")
}

func TestAccountService_RequestPasswordReset_LogsIgnoredUnknownOrIneligible(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		auth.NewJwtTokenProvider("test-secret"),
		auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache()),
		nil,
		nil,
		nil,
		0,
		repositories.passwordReset,
		&fixedPasswordResetTokenIssuer{tokens: []string{"reset-token-1"}},
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
		logger,
	)

	require.NoError(t, svc.RequestPasswordReset(context.Background(), "missing@example.com"))
	assert.Contains(t, logBuf.String(), `"event":"password_reset_request"`)
	assert.Contains(t, logBuf.String(), `"outcome":"ignored_unknown_or_ineligible"`)
}

func TestAccountService_ConfirmPasswordReset_LogsInvalidToken(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		auth.NewJwtTokenProvider("test-secret"),
		auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache()),
		nil,
		nil,
		nil,
		0,
		repositories.passwordReset,
		&fixedPasswordResetTokenIssuer{},
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
		logger,
	)

	err := svc.ConfirmPasswordReset(context.Background(), "missing-token", "newpw")
	require.Error(t, err)
	assert.ErrorIs(t, err, customerror.ErrInvalidToken)
	assert.Contains(t, logBuf.String(), `"event":"password_reset_confirm"`)
	assert.Contains(t, logBuf.String(), `"outcome":"invalid_token"`)
}

func TestAccountService_RequestEmailVerification_LogsAuditOutcome(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	_, err := userService.SignUp(context.Background(), "alice", "alice@example.com", "pw")
	require.NoError(t, err)
	user, err := repositories.user.SelectUserByUsername(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, user)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		auth.NewJwtTokenProvider("test-secret"),
		auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache()),
		repositories.emailVerification,
		&fixedEmailVerificationTokenIssuer{tokens: []string{"verify-token-1"}},
		newRecordingEmailVerificationMailSender(),
		30*time.Minute,
		repositories.passwordReset,
		&fixedPasswordResetTokenIssuer{},
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
		logger,
	)

	require.NoError(t, svc.RequestEmailVerification(context.Background(), user.ID))
	assert.Contains(t, logBuf.String(), `"event":"email_verification_request"`)
	assert.Contains(t, logBuf.String(), `"outcome":"issued"`)
	assert.Contains(t, logBuf.String(), `"user_id":1`)
	assert.NotContains(t, logBuf.String(), "alice@example.com")
}

func TestAccountService_RequestEmailVerification_LogsIgnoredUnknownOrIneligible(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		auth.NewJwtTokenProvider("test-secret"),
		auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache()),
		repositories.emailVerification,
		&fixedEmailVerificationTokenIssuer{tokens: []string{"verify-token-1"}},
		newRecordingEmailVerificationMailSender(),
		30*time.Minute,
		repositories.passwordReset,
		&fixedPasswordResetTokenIssuer{},
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
		logger,
	)

	require.NoError(t, svc.RequestEmailVerification(context.Background(), 999))
	assert.Contains(t, logBuf.String(), `"event":"email_verification_request"`)
	assert.Contains(t, logBuf.String(), `"outcome":"ignored_unknown_or_ineligible"`)
}

func TestAccountService_ConfirmEmailVerification_LogsInvalidToken(t *testing.T) {
	repositories := newTestRepositories()
	passwordHasher := newTestPasswordHasher()
	userService := NewUserService(repositories.user, passwordHasher, repositories.unitOfWork)
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	svc := NewAccountServiceWithGuestUpgrade(
		userService,
		&stubSessionUseCase{},
		repositories.user,
		repositories.unitOfWork,
		passwordHasher,
		auth.NewJwtTokenProvider("test-secret"),
		auth.NewCacheSessionRepository(cacheInMemory.NewInMemoryCache()),
		repositories.emailVerification,
		&fixedEmailVerificationTokenIssuer{},
		newRecordingEmailVerificationMailSender(),
		30*time.Minute,
		repositories.passwordReset,
		&fixedPasswordResetTokenIssuer{},
		newRecordingPasswordResetMailSender(),
		30*time.Minute,
		logger,
	)

	err := svc.ConfirmEmailVerification(context.Background(), "missing-token")
	require.Error(t, err)
	assert.ErrorIs(t, err, customerror.ErrInvalidToken)
	assert.Contains(t, logBuf.String(), `"event":"email_verification_confirm"`)
	assert.Contains(t, logBuf.String(), `"outcome":"invalid_token"`)
}
