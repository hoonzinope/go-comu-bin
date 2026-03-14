package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromViper_ValidConfig(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
	v.Set("delivery.http.maxJSONBodyBytes", int64(1<<20))
	v.Set("event.outbox.workerCount", 3)
	v.Set("event.outbox.batchSize", 200)
	v.Set("event.outbox.pollIntervalMillis", 50)
	v.Set("event.outbox.maxAttempts", 7)
	v.Set("event.outbox.baseBackoffMillis", 150)
	v.Set("admin.bootstrap.enabled", true)
	v.Set("admin.bootstrap.username", "admin")
	v.Set("admin.bootstrap.password", "strong-admin-password")
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
	assert.Equal(t, int64(1<<20), cfg.Delivery.HTTP.MaxJSONBodyBytes)
	assert.True(t, cfg.Delivery.HTTP.RateLimit.Enabled)
	assert.Equal(t, 60, cfg.Delivery.HTTP.RateLimit.WindowSeconds)
	assert.Equal(t, 60, cfg.Delivery.HTTP.RateLimit.WriteRequests)
	assert.Equal(t, 3, cfg.Event.Outbox.WorkerCount)
	assert.Equal(t, 200, cfg.Event.Outbox.BatchSize)
	assert.Equal(t, 50, cfg.Event.Outbox.PollIntervalMillis)
	assert.Equal(t, 7, cfg.Event.Outbox.MaxAttempts)
	assert.Equal(t, 150, cfg.Event.Outbox.BaseBackoffMillis)
	assert.Equal(t, 30, cfg.Cache.ListTTLSeconds)
	assert.Equal(t, 60, cfg.Cache.DetailTTLSeconds)
	assert.True(t, cfg.Admin.Bootstrap.Enabled)
	assert.Equal(t, "admin", cfg.Admin.Bootstrap.Username)
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
      secret: "test-secret-1234567890-abcdef-1234"

admin:
  bootstrap:
    enabled: true
    username: "admin"
    password: "strong-admin-password"

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
	assert.True(t, cfg.Admin.Bootstrap.Enabled)
}

func TestLoad_LoadsFromEnvironmentWithoutConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	t.Setenv("DELIVERY_HTTP_PORT", "18577")
	t.Setenv("DELIVERY_HTTP_AUTH_SECRET", "env-secret-1234567890-abcdef-1234")
	t.Setenv("STORAGE_PROVIDER", "local")
	t.Setenv("STORAGE_LOCAL_ROOTDIR", "./data/uploads")
	t.Setenv("STORAGE_ATTACHMENT_MAXUPLOADSIZEBYTES", "10485760")
	t.Setenv("STORAGE_ATTACHMENT_IMAGEOPTIMIZATION_JPEGQUALITY", "82")
	t.Setenv("JOBS_ATTACHMENTCLEANUP_INTERVALSECONDS", "600")
	t.Setenv("JOBS_ATTACHMENTCLEANUP_GRACEPERIODSECONDS", "600")
	t.Setenv("JOBS_ATTACHMENTCLEANUP_BATCHSIZE", "50")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 18577, cfg.Delivery.HTTP.Port)
	assert.Equal(t, "env-secret-1234567890-abcdef-1234", cfg.Delivery.HTTP.Auth.Secret)
}

func TestLoadFromViper_InvalidPort(t *testing.T) {
	t.Run("port_is_zero", func(t *testing.T) {
		v := viper.New()
		v.Set("delivery.http.port", 0)
		v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
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
		v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
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
	v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
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

func TestLoadFromViper_InvalidMaxJSONBodyBytes(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
	v.Set("delivery.http.maxJSONBodyBytes", int64(0))
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
}

func TestLoadFromViper_InvalidDefaultPageLimit(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
	v.Set("delivery.http.defaultPageLimit", 1001)
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
}

func TestLoadFromViper_InvalidRateLimitWindowSeconds(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
	v.Set("delivery.http.rateLimit.windowSeconds", 0)
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
}

func TestLoadFromViper_InvalidCacheTTL(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
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
	v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
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
	v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
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
	v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
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
	v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
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
	v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
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

func TestLoadFromViper_RejectsPlaceholderJWTSecret(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "commu-bin-secret-key")
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
}

func TestLoadFromViper_RejectsWhitespaceOnlyJWTSecret(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "   ")
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
}

func TestLoadFromViper_RejectsTooShortJWTSecret(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "short-secret")
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
}

func TestLoadFromViper_AllowsZeroCleanupConfigWhenJobsDisabled(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
	v.Set("cache.listTTLSeconds", 30)
	v.Set("cache.detailTTLSeconds", 30)
	v.Set("storage.provider", "local")
	v.Set("storage.local.rootDir", "./data/uploads")
	v.Set("storage.attachment.maxUploadSizeBytes", int64(10<<20))
	v.Set("storage.attachment.imageOptimization.jpegQuality", 82)
	v.Set("jobs.enabled", false)
	v.Set("jobs.attachmentCleanup.enabled", false)
	v.Set("jobs.attachmentCleanup.intervalSeconds", 0)
	v.Set("jobs.attachmentCleanup.gracePeriodSeconds", 0)
	v.Set("jobs.attachmentCleanup.batchSize", 0)

	cfg, err := loadFromViper(v)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.False(t, cfg.Jobs.Enabled)
}

func TestLoadFromViper_RequiresBootstrapCredentialsWhenEnabled(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
	v.Set("admin.bootstrap.enabled", true)
	v.Set("admin.bootstrap.username", "admin")
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
}

func TestLoadFromViper_InvalidOutboxConfig(t *testing.T) {
	base := func() *viper.Viper {
		v := viper.New()
		v.Set("delivery.http.port", 18577)
		v.Set("delivery.http.auth.secret", "test-secret-1234567890-abcdef-1234")
		v.Set("cache.listTTLSeconds", 30)
		v.Set("cache.detailTTLSeconds", 30)
		v.Set("storage.provider", "local")
		v.Set("storage.local.rootDir", "./data/uploads")
		v.Set("storage.attachment.maxUploadSizeBytes", int64(10<<20))
		v.Set("storage.attachment.imageOptimization.jpegQuality", 82)
		v.Set("jobs.attachmentCleanup.intervalSeconds", 600)
		v.Set("jobs.attachmentCleanup.gracePeriodSeconds", 600)
		v.Set("jobs.attachmentCleanup.batchSize", 50)
		v.Set("event.outbox.workerCount", 1)
		v.Set("event.outbox.batchSize", 100)
		v.Set("event.outbox.pollIntervalMillis", 100)
		v.Set("event.outbox.maxAttempts", 5)
		v.Set("event.outbox.baseBackoffMillis", 100)
		return v
	}
	tests := []struct {
		name      string
		field     string
		value     int
		errSubstr string
	}{
		{name: "worker count", field: "event.outbox.workerCount", value: 0, errSubstr: "event.outbox.workerCount"},
		{name: "batch size", field: "event.outbox.batchSize", value: 0, errSubstr: "event.outbox.batchSize"},
		{name: "poll interval", field: "event.outbox.pollIntervalMillis", value: 0, errSubstr: "event.outbox.pollIntervalMillis"},
		{name: "max attempts", field: "event.outbox.maxAttempts", value: 0, errSubstr: "event.outbox.maxAttempts"},
		{name: "base backoff", field: "event.outbox.baseBackoffMillis", value: 0, errSubstr: "event.outbox.baseBackoffMillis"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := base()
			v.Set(tc.field, tc.value)

			cfg, err := loadFromViper(v)
			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.True(t, strings.Contains(err.Error(), tc.errSubstr))
		})
	}
}
