package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromViper_ValidConfig(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret")
	v.Set("cache.listTTLSeconds", 30)
	v.Set("cache.detailTTLSeconds", 60)
	v.Set("storage.provider", "local")
	v.Set("storage.local.rootDir", "./data/uploads")
	v.Set("storage.attachment.maxUploadSizeBytes", int64(10<<20))
	v.Set("storage.attachment.imageOptimization.enabled", true)
	v.Set("storage.attachment.imageOptimization.jpegQuality", 82)
	v.Set("jobs.enabled", true)
	v.Set("jobs.attachmentCleanup.enabled", true)
	v.Set("jobs.attachmentCleanup.intervalSeconds", 600)
	v.Set("jobs.attachmentCleanup.gracePeriodSeconds", 600)
	v.Set("jobs.attachmentCleanup.batchSize", 50)

	cfg, err := loadFromViper(v)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 18577, cfg.Delivery.HTTP.Port)
	assert.Equal(t, 30, cfg.Cache.ListTTLSeconds)
	assert.Equal(t, 60, cfg.Cache.DetailTTLSeconds)
	assert.Equal(t, "local", cfg.Storage.Provider)
	assert.Equal(t, "./data/uploads", cfg.Storage.Local.RootDir)
	assert.Equal(t, int64(10<<20), cfg.Storage.Attachment.MaxUploadSizeBytes)
	assert.True(t, cfg.Storage.Attachment.ImageOptimization.Enabled)
	assert.Equal(t, 82, cfg.Storage.Attachment.ImageOptimization.JPEGQuality)
	assert.True(t, cfg.Jobs.Enabled)
	assert.True(t, cfg.Jobs.AttachmentCleanup.Enabled)
	assert.Equal(t, 600, cfg.Jobs.AttachmentCleanup.IntervalSeconds)
	assert.Equal(t, 600, cfg.Jobs.AttachmentCleanup.GracePeriodSeconds)
}

func TestLoad_LoadsConfigFileFromWorkingDirectory(t *testing.T) {
	tempDir := t.TempDir()
	configBody := []byte(`cache:
  listTTLSeconds: 30
  detailTTLSeconds: 60

storage:
  provider: "local"
  local:
    rootDir: "./data/uploads"
  object:
    endpoint: ""
    bucket: ""
    accessKey: ""
    secretKey: ""
    useSSL: false
  attachment:
    maxUploadSizeBytes: 10485760
    imageOptimization:
      enabled: true
      jpegQuality: 82

delivery:
  http:
    port: 18577
    auth:
      secret: "test-secret"

jobs:
  enabled: true
  attachmentCleanup:
    enabled: true
    intervalSeconds: 600
    gracePeriodSeconds: 600
    batchSize: 50
`)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "config.yml"), configBody, 0o644))

	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 18577, cfg.Delivery.HTTP.Port)
	assert.True(t, cfg.Jobs.AttachmentCleanup.Enabled)
	assert.Equal(t, 600, cfg.Jobs.AttachmentCleanup.GracePeriodSeconds)
}

func TestLoadFromViper_InvalidPort(t *testing.T) {
	t.Run("port_is_zero", func(t *testing.T) {
		v := viper.New()
		v.Set("delivery.http.port", 0)
		v.Set("delivery.http.auth.secret", "test-secret")
		v.Set("cache.listTTLSeconds", 30)
		v.Set("cache.detailTTLSeconds", 30)
		v.Set("storage.provider", "local")
		v.Set("storage.local.rootDir", "./data/uploads")
		v.Set("storage.attachment.maxUploadSizeBytes", int64(10<<20))
		v.Set("storage.attachment.imageOptimization.jpegQuality", 82)
		v.Set("jobs.attachmentCleanup.intervalSeconds", 600)
		v.Set("jobs.attachmentCleanup.gracePeriodSeconds", 600)
		v.Set("jobs.attachmentCleanup.batchSize", 50)

		cfg, err := loadFromViper(v)
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("port_is_out_of_range", func(t *testing.T) {
		v := viper.New()
		v.Set("delivery.http.port", 70000)
		v.Set("delivery.http.auth.secret", "test-secret")
		v.Set("cache.listTTLSeconds", 30)
		v.Set("cache.detailTTLSeconds", 30)
		v.Set("storage.provider", "local")
		v.Set("storage.local.rootDir", "./data/uploads")
		v.Set("storage.attachment.maxUploadSizeBytes", int64(10<<20))
		v.Set("storage.attachment.imageOptimization.jpegQuality", 82)
		v.Set("jobs.attachmentCleanup.intervalSeconds", 600)
		v.Set("jobs.attachmentCleanup.gracePeriodSeconds", 600)
		v.Set("jobs.attachmentCleanup.batchSize", 50)

		cfg, err := loadFromViper(v)
		require.Error(t, err)
		assert.Nil(t, cfg)
	})
}

func TestLoadFromViper_UnknownField(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret")
	v.Set("cache.listTTLSeconds", 30)
	v.Set("cache.detailTTLSeconds", 30)
	v.Set("storage.provider", "local")
	v.Set("storage.local.rootDir", "./data/uploads")
	v.Set("storage.attachment.maxUploadSizeBytes", int64(10<<20))
	v.Set("storage.attachment.imageOptimization.jpegQuality", 82)
	v.Set("jobs.attachmentCleanup.intervalSeconds", 600)
	v.Set("jobs.attachmentCleanup.gracePeriodSeconds", 600)
	v.Set("jobs.attachmentCleanup.batchSize", 50)
	v.Set("delivery.http.unknown", true)

	cfg, err := loadFromViper(v)
	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadFromViper_InvalidCacheTTL(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret")
	v.Set("cache.listTTLSeconds", 0)
	v.Set("cache.detailTTLSeconds", 30)
	v.Set("storage.provider", "local")
	v.Set("storage.local.rootDir", "./data/uploads")
	v.Set("storage.attachment.maxUploadSizeBytes", int64(10<<20))
	v.Set("storage.attachment.imageOptimization.jpegQuality", 82)
	v.Set("jobs.attachmentCleanup.intervalSeconds", 600)
	v.Set("jobs.attachmentCleanup.gracePeriodSeconds", 600)
	v.Set("jobs.attachmentCleanup.batchSize", 50)

	cfg, err := loadFromViper(v)
	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadFromViper_InvalidStorageRoot(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret")
	v.Set("cache.listTTLSeconds", 30)
	v.Set("cache.detailTTLSeconds", 30)
	v.Set("storage.provider", "local")
	v.Set("storage.local.rootDir", "")
	v.Set("storage.attachment.maxUploadSizeBytes", int64(10<<20))
	v.Set("storage.attachment.imageOptimization.jpegQuality", 82)
	v.Set("jobs.attachmentCleanup.intervalSeconds", 600)
	v.Set("jobs.attachmentCleanup.gracePeriodSeconds", 600)
	v.Set("jobs.attachmentCleanup.batchSize", 50)

	cfg, err := loadFromViper(v)
	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadFromViper_InvalidAttachmentMaxUploadSize(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret")
	v.Set("cache.listTTLSeconds", 30)
	v.Set("cache.detailTTLSeconds", 30)
	v.Set("storage.provider", "local")
	v.Set("storage.local.rootDir", "./data/uploads")
	v.Set("storage.attachment.maxUploadSizeBytes", int64(0))
	v.Set("storage.attachment.imageOptimization.jpegQuality", 82)
	v.Set("jobs.attachmentCleanup.intervalSeconds", 600)
	v.Set("jobs.attachmentCleanup.gracePeriodSeconds", 600)
	v.Set("jobs.attachmentCleanup.batchSize", 50)

	cfg, err := loadFromViper(v)
	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadFromViper_InvalidAttachmentJPEGQuality(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret")
	v.Set("cache.listTTLSeconds", 30)
	v.Set("cache.detailTTLSeconds", 30)
	v.Set("storage.provider", "local")
	v.Set("storage.local.rootDir", "./data/uploads")
	v.Set("storage.attachment.maxUploadSizeBytes", int64(10<<20))
	v.Set("storage.attachment.imageOptimization.jpegQuality", 101)
	v.Set("jobs.attachmentCleanup.intervalSeconds", 600)
	v.Set("jobs.attachmentCleanup.gracePeriodSeconds", 600)
	v.Set("jobs.attachmentCleanup.batchSize", 50)

	cfg, err := loadFromViper(v)
	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadFromViper_InvalidAttachmentCleanupInterval(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret")
	v.Set("cache.listTTLSeconds", 30)
	v.Set("cache.detailTTLSeconds", 30)
	v.Set("storage.provider", "local")
	v.Set("storage.local.rootDir", "./data/uploads")
	v.Set("storage.attachment.maxUploadSizeBytes", int64(10<<20))
	v.Set("storage.attachment.imageOptimization.jpegQuality", 82)
	v.Set("jobs.attachmentCleanup.intervalSeconds", 0)
	v.Set("jobs.attachmentCleanup.gracePeriodSeconds", 600)
	v.Set("jobs.attachmentCleanup.batchSize", 50)

	cfg, err := loadFromViper(v)
	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadFromViper_ObjectStorageConfig(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret")
	v.Set("cache.listTTLSeconds", 30)
	v.Set("cache.detailTTLSeconds", 30)
	v.Set("storage.provider", "object")
	v.Set("storage.object.endpoint", "localhost:9000")
	v.Set("storage.object.bucket", "attachments")
	v.Set("storage.object.accessKey", "minio")
	v.Set("storage.object.secretKey", "minio123")
	v.Set("storage.object.useSSL", false)
	v.Set("storage.attachment.maxUploadSizeBytes", int64(10<<20))
	v.Set("storage.attachment.imageOptimization.jpegQuality", 82)
	v.Set("jobs.attachmentCleanup.intervalSeconds", 600)
	v.Set("jobs.attachmentCleanup.gracePeriodSeconds", 600)
	v.Set("jobs.attachmentCleanup.batchSize", 50)

	cfg, err := loadFromViper(v)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "object", cfg.Storage.Provider)
	assert.Equal(t, "attachments", cfg.Storage.Object.Bucket)
}
