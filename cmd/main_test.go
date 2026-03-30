package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/config"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubUserRepository struct {
	selectUserByUsername                 func(ctx context.Context, username string) (*entity.User, error)
	selectUserByUsernameIncludingDeleted func(ctx context.Context, username string) (*entity.User, error)
	save                                 func(ctx context.Context, user *entity.User) (int64, error)
	update                               func(ctx context.Context, user *entity.User) error
}

func (r *stubUserRepository) Save(ctx context.Context, user *entity.User) (int64, error) {
	if r.save != nil {
		return r.save(ctx, user)
	}
	return 1, nil
}

func (r *stubUserRepository) SelectUserByUsername(ctx context.Context, username string) (*entity.User, error) {
	if r.selectUserByUsername != nil {
		return r.selectUserByUsername(ctx, username)
	}
	return nil, nil
}

func (r *stubUserRepository) SelectUserByUsernameIncludingDeleted(ctx context.Context, username string) (*entity.User, error) {
	if r.selectUserByUsernameIncludingDeleted != nil {
		return r.selectUserByUsernameIncludingDeleted(ctx, username)
	}
	return nil, nil
}

func (r *stubUserRepository) SelectUserByEmail(context.Context, string) (*entity.User, error) {
	return nil, nil
}

func (r *stubUserRepository) SelectUserByUUID(context.Context, string) (*entity.User, error) {
	return nil, nil
}

func (r *stubUserRepository) SelectUserByID(context.Context, int64) (*entity.User, error) {
	return nil, nil
}

func (r *stubUserRepository) SelectUserByIDIncludingDeleted(context.Context, int64) (*entity.User, error) {
	return nil, nil
}

func (r *stubUserRepository) SelectUsersByIDsIncludingDeleted(context.Context, []int64) (map[int64]*entity.User, error) {
	return map[int64]*entity.User{}, nil
}

func (r *stubUserRepository) SelectGuestCleanupCandidates(context.Context, time.Time, time.Duration, time.Duration, int) ([]*entity.User, error) {
	return []*entity.User{}, nil
}

func (r *stubUserRepository) Update(ctx context.Context, user *entity.User) error {
	if r.update != nil {
		return r.update(ctx, user)
	}
	return nil
}

func (r *stubUserRepository) Delete(context.Context, int64) error {
	return nil
}

func TestEnsureBootstrapAdmin_ReturnsError_WhenSaveFails(t *testing.T) {
	expected := errors.New("save failed")
	cfg := &config.Config{}
	cfg.Admin.Bootstrap.Enabled = true
	cfg.Admin.Bootstrap.Username = "admin"
	cfg.Admin.Bootstrap.Password = "strong-admin-password"

	err := ensureBootstrapAdmin(cfg, &stubUserRepository{
		save: func(ctx context.Context, user *entity.User) (int64, error) {
			_ = ctx
			require.Equal(t, "admin", user.Name)
			require.NotEmpty(t, user.Password)
			return 0, expected
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, expected)
}

func TestEnsureBootstrapAdmin_SkipsWhenDisabled(t *testing.T) {
	cfg := &config.Config{}
	called := false

	err := ensureBootstrapAdmin(cfg, &stubUserRepository{
		save: func(context.Context, *entity.User) (int64, error) {
			called = true
			return 1, nil
		},
	})

	require.NoError(t, err)
	assert.False(t, called)
}

func TestEnsureBootstrapAdmin_UpdatesExistingActiveUser(t *testing.T) {
	cfg := &config.Config{}
	cfg.Admin.Bootstrap.Enabled = true
	cfg.Admin.Bootstrap.Username = "admin"
	cfg.Admin.Bootstrap.Password = "strong-admin-password"

	var updatedUser *entity.User
	err := ensureBootstrapAdmin(cfg, &stubUserRepository{
		selectUserByUsernameIncludingDeleted: func(_ context.Context, username string) (*entity.User, error) {
			return &entity.User{ID: 1, Name: username, Role: "user", Status: entity.UserStatusActive}, nil
		},
		update: func(_ context.Context, user *entity.User) error {
			updatedUser = user
			return nil
		},
		save: func(context.Context, *entity.User) (int64, error) {
			t.Fatal("save should not be called when updating an existing user")
			return 0, nil
		},
	})

	require.NoError(t, err)
	require.NotNil(t, updatedUser)
	assert.Equal(t, "admin", updatedUser.Name)
	assert.Equal(t, "admin", updatedUser.Role)
	assert.Equal(t, entity.UserStatusActive, updatedUser.Status)
}

func TestEnsureBootstrapAdmin_UpsertsExistingUser(t *testing.T) {
	cfg := &config.Config{}
	cfg.Admin.Bootstrap.Enabled = true
	cfg.Admin.Bootstrap.Username = "admin"
	cfg.Admin.Bootstrap.Password = "admin"

	var updatedUser *entity.User
	err := ensureBootstrapAdmin(cfg, &stubUserRepository{
		selectUserByUsernameIncludingDeleted: func(_ context.Context, username string) (*entity.User, error) {
			return &entity.User{
				ID:               1,
				UUID:             "existing-uuid",
				Name:             username,
				Password:         "old-password",
				Role:             "user",
				Status:           entity.UserStatusDeleted,
				DeletedAt:        func() *time.Time { now := time.Now().Add(-time.Hour); return &now }(),
				CreatedAt:        time.Now().Add(-time.Hour),
				UpdatedAt:        time.Now().Add(-time.Hour),
				SuspensionReason: "old",
			}, nil
		},
		update: func(_ context.Context, user *entity.User) error {
			updatedUser = user
			return nil
		},
		save: func(context.Context, *entity.User) (int64, error) {
			t.Fatal("save should not be called when upserting existing user")
			return 0, nil
		},
	})

	require.NoError(t, err)
	require.NotNil(t, updatedUser)
	assert.Equal(t, int64(1), updatedUser.ID)
	assert.Equal(t, "admin", updatedUser.Name)
	assert.Equal(t, "admin", updatedUser.Role)
	assert.Equal(t, entity.UserStatusActive, updatedUser.Status)
	assert.Nil(t, updatedUser.DeletedAt)
	assert.NotEqual(t, "old-password", updatedUser.Password)
}

type stubAttachmentCleanupUseCase struct{}

func (s stubAttachmentCleanupUseCase) CleanupAttachments(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error) {
	return 0, nil
}

type recordingAttachmentCleanupUseCase struct {
	lastCtx context.Context
	called  chan struct{}
}

func (s *recordingAttachmentCleanupUseCase) CleanupAttachments(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error) {
	s.lastCtx = ctx
	if s.called != nil {
		select {
		case s.called <- struct{}{}:
		default:
		}
	}
	return 0, nil
}

type stubGuestCleanupUseCase struct{}

func (s stubGuestCleanupUseCase) CleanupGuests(ctx context.Context, now time.Time, pendingGrace, activeUnusedGrace time.Duration, limit int) (int, error) {
	return 0, nil
}

type recordingGuestCleanupUseCase struct {
	lastCtx context.Context
	called  chan struct{}
}

func (s *recordingGuestCleanupUseCase) CleanupGuests(ctx context.Context, now time.Time, pendingGrace, activeUnusedGrace time.Duration, limit int) (int, error) {
	s.lastCtx = ctx
	if s.called != nil {
		select {
		case s.called <- struct{}{}:
		default:
		}
	}
	return 0, nil
}

type stubEmailVerificationCleanupUseCase struct{}

func (s stubEmailVerificationCleanupUseCase) CleanupEmailVerificationTokens(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error) {
	return 0, nil
}

type recordingEmailVerificationCleanupUseCase struct {
	lastCtx context.Context
	called  chan struct{}
}

func (s *recordingEmailVerificationCleanupUseCase) CleanupEmailVerificationTokens(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error) {
	s.lastCtx = ctx
	if s.called != nil {
		select {
		case s.called <- struct{}{}:
		default:
		}
	}
	return 0, nil
}

type stubPasswordResetCleanupUseCase struct{}

func (s stubPasswordResetCleanupUseCase) CleanupPasswordResetTokens(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error) {
	return 0, nil
}

type recordingPasswordResetCleanupUseCase struct {
	lastCtx context.Context
	called  chan struct{}
}

func (s *recordingPasswordResetCleanupUseCase) CleanupPasswordResetTokens(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error) {
	s.lastCtx = ctx
	if s.called != nil {
		select {
		case s.called <- struct{}{}:
		default:
		}
	}
	return 0, nil
}

func TestMainHelpers(t *testing.T) {
	cfg := &config.Config{}
	cfg.Delivery.HTTP.Port = 18577
	cfg.Delivery.HTTP.Auth.Secret = "secret"
	cfg.Cache.ListTTLSeconds = 30
	cfg.Cache.DetailTTLSeconds = 45
	cfg.Storage.Provider = "local"
	cfg.Storage.Local.RootDir = "/tmp/uploads"

	assert.Equal(t, ":18577", httpAddr(cfg))
	assert.Equal(t, "secret", jwtSecret(cfg))
	assert.Equal(t, appcache.Policy{ListTTLSeconds: 30, DetailTTLSeconds: 45}, cachePolicy(cfg))
}

func TestNewFileStorage(t *testing.T) {
	cfg := &config.Config{}
	cfg.Storage.Provider = "local"
	cfg.Storage.Local.RootDir = "/tmp/uploads"

	storage, err := newFileStorage(cfg)
	require.NoError(t, err)
	assert.NotNil(t, storage)

	cfg.Storage.Provider = "object"
	cfg.Storage.Object.Endpoint = "localhost:9000"
	cfg.Storage.Object.Bucket = "bucket"
	cfg.Storage.Object.AccessKey = "key"
	cfg.Storage.Object.SecretKey = "secret"
	objectStorage, err := newFileStorage(cfg)
	require.NoError(t, err)
	assert.NotNil(t, objectStorage)

	cfg.Storage.Provider = "unknown"
	_, err = newFileStorage(cfg)
	require.Error(t, err)
}

func TestNewAppLogger_WritesToStdoutAndFile(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{}
	cfg.Logging.FilePath = filepath.Join(tempDir, "app.jsonl")
	cfg.Logging.MaxSizeMB = 1
	cfg.Logging.MaxBackups = 1
	cfg.Logging.MaxAgeDays = 1
	cfg.Logging.Compress = false
	cfg.Logging.LocalTime = true

	var stdout bytes.Buffer
	logger, closer, err := newAppLogger(&stdout, cfg)
	require.NoError(t, err)
	require.NotNil(t, logger)
	require.NotNil(t, closer)

	logger.Info("hello", "k", "v")
	require.NoError(t, closer.Close())

	fileContent, err := os.ReadFile(cfg.Logging.FilePath)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"msg":"hello"`)
	assert.Contains(t, stdout.String(), `"k":"v"`)
	assert.Contains(t, string(fileContent), `"msg":"hello"`)
	assert.Contains(t, string(fileContent), `"k":"v"`)
}

func TestNewAppLogger_RotatesFiles(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{}
	cfg.Logging.FilePath = filepath.Join(tempDir, "app.jsonl")
	cfg.Logging.MaxSizeMB = 1
	cfg.Logging.MaxBackups = 2
	cfg.Logging.MaxAgeDays = 1
	cfg.Logging.Compress = false
	cfg.Logging.LocalTime = true

	logger, closer, err := newAppLogger(io.Discard, cfg)
	require.NoError(t, err)

	payload := strings.Repeat("x", 20_000)
	for i := 0; i < 80; i++ {
		logger.Info("rotation-test", "payload", payload, "seq", i)
	}
	require.NoError(t, closer.Close())

	matches, err := filepath.Glob(filepath.Join(tempDir, "app*"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(matches), 2)
}

func TestStartBackgroundJobs_ReturnsNilWhenDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Jobs.Enabled = false

	err := startBackgroundJobs(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, stubAttachmentCleanupUseCase{}, stubGuestCleanupUseCase{}, stubEmailVerificationCleanupUseCase{}, stubPasswordResetCleanupUseCase{})
	require.NoError(t, err)
}

func TestStartBackgroundJobs_ReturnsNilWhenCleanupJobDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Jobs.Enabled = true
	cfg.Jobs.AttachmentCleanup.Enabled = false

	err := startBackgroundJobs(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, stubAttachmentCleanupUseCase{}, stubGuestCleanupUseCase{}, stubEmailVerificationCleanupUseCase{}, stubPasswordResetCleanupUseCase{})
	require.NoError(t, err)
}

func TestStartBackgroundJobs_PassesParentContextToCleanupUseCase(t *testing.T) {
	cfg := &config.Config{}
	cfg.Jobs.Enabled = true
	cfg.Jobs.AttachmentCleanup.Enabled = true
	cfg.Jobs.AttachmentCleanup.IntervalSeconds = 1
	cfg.Jobs.AttachmentCleanup.GracePeriodSeconds = 10
	cfg.Jobs.AttachmentCleanup.BatchSize = 5

	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	recorder := &recordingAttachmentCleanupUseCase{called: make(chan struct{}, 1)}

	err := startBackgroundJobs(parentCtx, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, recorder, stubGuestCleanupUseCase{}, stubEmailVerificationCleanupUseCase{}, stubPasswordResetCleanupUseCase{})
	require.NoError(t, err)

	select {
	case <-recorder.called:
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("cleanup job was not triggered")
	}

	require.NotNil(t, recorder.lastCtx)
	assert.Same(t, parentCtx, recorder.lastCtx)
}

func TestStartBackgroundJobs_PassesParentContextToGuestCleanupUseCase(t *testing.T) {
	cfg := &config.Config{}
	cfg.Jobs.Enabled = true
	cfg.Jobs.AttachmentCleanup.Enabled = false
	cfg.Jobs.GuestCleanup.Enabled = true
	cfg.Jobs.GuestCleanup.IntervalSeconds = 1
	cfg.Jobs.GuestCleanup.PendingGracePeriodSeconds = 10
	cfg.Jobs.GuestCleanup.ActiveUnusedGracePeriodSeconds = 60
	cfg.Jobs.GuestCleanup.BatchSize = 5

	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	recorder := &recordingGuestCleanupUseCase{called: make(chan struct{}, 1)}

	err := startBackgroundJobs(parentCtx, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, stubAttachmentCleanupUseCase{}, recorder, stubEmailVerificationCleanupUseCase{}, stubPasswordResetCleanupUseCase{})
	require.NoError(t, err)

	select {
	case <-recorder.called:
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("guest cleanup job was not triggered")
	}

	require.NotNil(t, recorder.lastCtx)
	assert.Same(t, parentCtx, recorder.lastCtx)
}

func TestStartBackgroundJobs_PassesParentContextToPasswordResetCleanupUseCase(t *testing.T) {
	cfg := &config.Config{}
	cfg.Jobs.Enabled = true
	cfg.Jobs.AttachmentCleanup.Enabled = false
	cfg.Jobs.GuestCleanup.Enabled = false
	cfg.Jobs.PasswordResetCleanup.Enabled = true
	cfg.Jobs.PasswordResetCleanup.IntervalSeconds = 1
	cfg.Jobs.PasswordResetCleanup.GracePeriodSeconds = 60
	cfg.Jobs.PasswordResetCleanup.BatchSize = 5

	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	recorder := &recordingPasswordResetCleanupUseCase{called: make(chan struct{}, 1)}

	err := startBackgroundJobs(parentCtx, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, stubAttachmentCleanupUseCase{}, stubGuestCleanupUseCase{}, stubEmailVerificationCleanupUseCase{}, recorder)
	require.NoError(t, err)

	select {
	case <-recorder.called:
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("password reset cleanup job was not triggered")
	}

	require.NotNil(t, recorder.lastCtx)
	assert.Same(t, parentCtx, recorder.lastCtx)
}

func TestStartBackgroundJobs_PassesParentContextToEmailVerificationCleanupUseCase(t *testing.T) {
	cfg := &config.Config{}
	cfg.Jobs.Enabled = true
	cfg.Jobs.AttachmentCleanup.Enabled = false
	cfg.Jobs.GuestCleanup.Enabled = false
	cfg.Jobs.EmailVerificationCleanup.Enabled = true
	cfg.Jobs.EmailVerificationCleanup.IntervalSeconds = 1
	cfg.Jobs.EmailVerificationCleanup.GracePeriodSeconds = 60
	cfg.Jobs.EmailVerificationCleanup.BatchSize = 5

	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	recorder := &recordingEmailVerificationCleanupUseCase{called: make(chan struct{}, 1)}

	err := startBackgroundJobs(parentCtx, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, stubAttachmentCleanupUseCase{}, stubGuestCleanupUseCase{}, recorder, stubPasswordResetCleanupUseCase{})
	require.NoError(t, err)

	select {
	case <-recorder.called:
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("email verification cleanup job was not triggered")
	}

	require.NotNil(t, recorder.lastCtx)
	assert.Same(t, parentCtx, recorder.lastCtx)
}

type stubHTTPShutdowner struct {
	shutdownCalled int32
	closeCalled    int32
	shutdownErr    error
}

func (s *stubHTTPShutdowner) Shutdown(ctx context.Context) error {
	atomic.AddInt32(&s.shutdownCalled, 1)
	return s.shutdownErr
}

func (s *stubHTTPShutdowner) Close() error {
	atomic.AddInt32(&s.closeCalled, 1)
	return nil
}

type stubRelayWaiter struct {
	called int32
}

func (s *stubRelayWaiter) Wait() {
	atomic.AddInt32(&s.called, 1)
}

type blockingRelayWaiter struct {
	called int32
	until  <-chan struct{}
}

func (s *blockingRelayWaiter) Wait() {
	atomic.AddInt32(&s.called, 1)
	<-s.until
}

func TestGracefulShutdown_CallsServerShutdownAndRelayWait(t *testing.T) {
	server := &stubHTTPShutdowner{}
	relay := &stubRelayWaiter{}
	cancelCalled := int32(0)
	cancel := func() { atomic.AddInt32(&cancelCalled, 1) }
	serverErrCh := make(chan error, 1)
	serverErrCh <- errors.New("stopped")

	_ = gracefulShutdown(
		server,
		serverErrCh,
		relay,
		cancel,
		50*time.Millisecond,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	assert.Equal(t, int32(1), atomic.LoadInt32(&cancelCalled))
	assert.Equal(t, int32(1), atomic.LoadInt32(&server.shutdownCalled))
	assert.Equal(t, int32(0), atomic.LoadInt32(&server.closeCalled))
	assert.Equal(t, int32(1), atomic.LoadInt32(&relay.called))
}

func TestGracefulShutdown_FallbackCloseWhenServerDoesNotExit(t *testing.T) {
	server := &stubHTTPShutdowner{shutdownErr: errors.New("shutdown timeout")}
	relay := &stubRelayWaiter{}
	cancelCalled := int32(0)
	cancel := func() { atomic.AddInt32(&cancelCalled, 1) }
	serverErrCh := make(chan error) // never receives

	err := gracefulShutdown(
		server,
		serverErrCh,
		relay,
		cancel,
		20*time.Millisecond,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
	assert.Equal(t, int32(1), atomic.LoadInt32(&server.shutdownCalled))
	assert.Equal(t, int32(1), atomic.LoadInt32(&server.closeCalled))
	assert.Equal(t, int32(1), atomic.LoadInt32(&cancelCalled))
	assert.Equal(t, int32(1), atomic.LoadInt32(&relay.called))
}

func TestGracefulShutdown_CancelsBeforeWaitingRelay(t *testing.T) {
	server := &stubHTTPShutdowner{}
	unblockRelay := make(chan struct{})
	relay := &blockingRelayWaiter{until: unblockRelay}
	cancelCalled := int32(0)
	cancel := func() {
		atomic.AddInt32(&cancelCalled, 1)
		close(unblockRelay)
	}
	serverErrCh := make(chan error, 1)
	serverErrCh <- errors.New("stopped")

	done := make(chan error, 1)
	go func() {
		done <- gracefulShutdown(
			server,
			serverErrCh,
			relay,
			cancel,
			100*time.Millisecond,
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.Equal(t, int32(1), atomic.LoadInt32(&cancelCalled))
		assert.Equal(t, int32(1), atomic.LoadInt32(&relay.called))
	case <-time.After(300 * time.Millisecond):
		t.Fatal("gracefulShutdown did not return; likely waiting relay before cancel")
	}
}

func TestGracefulShutdown_UsesSingleTimeoutBudget(t *testing.T) {
	server := &stubHTTPShutdowner{}
	relay := &stubRelayWaiter{}
	serverErrCh := make(chan error) // never receives
	timeout := 40 * time.Millisecond
	start := time.Now()

	err := gracefulShutdown(
		server,
		serverErrCh,
		relay,
		func() {},
		timeout,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
	assert.LessOrEqual(t, time.Since(start), timeout+60*time.Millisecond)
}
