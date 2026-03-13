package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
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
	selectUserByUsername func(ctx context.Context, username string) (*entity.User, error)
	save                 func(ctx context.Context, user *entity.User) (int64, error)
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

func (r *stubUserRepository) Update(context.Context, *entity.User) error {
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

func TestEnsureBootstrapAdmin_SkipsWhenUserAlreadyExists(t *testing.T) {
	cfg := &config.Config{}
	cfg.Admin.Bootstrap.Enabled = true
	cfg.Admin.Bootstrap.Username = "admin"
	cfg.Admin.Bootstrap.Password = "strong-admin-password"

	calledSave := false
	err := ensureBootstrapAdmin(cfg, &stubUserRepository{
		selectUserByUsername: func(_ context.Context, username string) (*entity.User, error) {
			return &entity.User{ID: 1, Name: username}, nil
		},
		save: func(context.Context, *entity.User) (int64, error) {
			calledSave = true
			return 1, nil
		},
	})

	require.NoError(t, err)
	assert.False(t, calledSave)
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

func TestStartBackgroundJobs_ReturnsNilWhenDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Jobs.Enabled = false

	err := startBackgroundJobs(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, stubAttachmentCleanupUseCase{})
	require.NoError(t, err)
}

func TestStartBackgroundJobs_ReturnsNilWhenCleanupJobDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Jobs.Enabled = true
	cfg.Jobs.AttachmentCleanup.Enabled = false

	err := startBackgroundJobs(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, stubAttachmentCleanupUseCase{})
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

	err := startBackgroundJobs(parentCtx, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, recorder)
	require.NoError(t, err)

	select {
	case <-recorder.called:
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("cleanup job was not triggered")
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
