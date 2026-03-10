package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/config"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubUserRepository struct {
	save func(user *entity.User) (int64, error)
}

func (r *stubUserRepository) Save(user *entity.User) (int64, error) {
	if r.save != nil {
		return r.save(user)
	}
	return 1, nil
}

func (r *stubUserRepository) SelectUserByUsername(username string) (*entity.User, error) {
	return nil, nil
}

func (r *stubUserRepository) SelectUserByUUID(userUUID string) (*entity.User, error) {
	return nil, nil
}

func (r *stubUserRepository) SelectUserByID(id int64) (*entity.User, error) {
	return nil, nil
}

func (r *stubUserRepository) SelectUserByIDIncludingDeleted(id int64) (*entity.User, error) {
	return nil, nil
}

func (r *stubUserRepository) SelectUsersByIDsIncludingDeleted(ids []int64) (map[int64]*entity.User, error) {
	return map[int64]*entity.User{}, nil
}

func (r *stubUserRepository) Update(user *entity.User) error {
	return nil
}

func (r *stubUserRepository) Delete(id int64) error {
	return nil
}

func TestSeedAdmin_ReturnsError_WhenSaveFails(t *testing.T) {
	expected := errors.New("save failed")

	err := seedAdmin(&stubUserRepository{
		save: func(user *entity.User) (int64, error) {
			require.Equal(t, "admin", user.Name)
			require.NotEmpty(t, user.Password)
			return 0, expected
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, expected)
}

type stubAttachmentCleanupUseCase struct{}

func (s stubAttachmentCleanupUseCase) CleanupAttachments(ctx context.Context, now time.Time, gracePeriod time.Duration, limit int) (int, error) {
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
