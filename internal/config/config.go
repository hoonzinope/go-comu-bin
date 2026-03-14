package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

const placeholderJWTSecret = "commu-bin-secret-key"
const placeholderBootstrapPassword = "admin"
const minJWTSecretLength = 32
const minDefaultPageLimit = 1
const maxDefaultPageLimit = 1000
const minRateLimitWindowSeconds = 1
const minRateLimitWriteRequests = 1

type Config struct {
	Cache struct {
		ListTTLSeconds   int `yaml:"listTTLSeconds"`
		DetailTTLSeconds int `yaml:"detailTTLSeconds"`
	} `yaml:"cache"`
	Admin struct {
		Bootstrap struct {
			Enabled  bool   `yaml:"enabled"`
			Username string `yaml:"username"`
			Password string `yaml:"password"`
		} `yaml:"bootstrap"`
	} `yaml:"admin"`
	Storage struct {
		Provider string `yaml:"provider"`
		Local    struct {
			RootDir string `yaml:"rootDir"`
		} `yaml:"local"`
		Object struct {
			Endpoint  string `yaml:"endpoint"`
			Bucket    string `yaml:"bucket"`
			AccessKey string `yaml:"accessKey"`
			SecretKey string `yaml:"secretKey"`
			UseSSL    bool   `yaml:"useSSL"`
		} `yaml:"object"`
		Attachment struct {
			MaxUploadSizeBytes int64 `yaml:"maxUploadSizeBytes"`
			ImageOptimization  struct {
				Enabled     bool `yaml:"enabled"`
				JPEGQuality int  `yaml:"jpegQuality"`
			} `yaml:"imageOptimization"`
		} `yaml:"attachment"`
	} `yaml:"storage"`
	Delivery struct {
		HTTP struct {
			Port             int   `yaml:"port"`
			MaxJSONBodyBytes int64 `yaml:"maxJSONBodyBytes"`
			DefaultPageLimit int   `yaml:"defaultPageLimit"`
			RateLimit        struct {
				Enabled       bool `yaml:"enabled"`
				WindowSeconds int  `yaml:"windowSeconds"`
				WriteRequests int  `yaml:"writeRequests"`
			} `yaml:"rateLimit"`
			Auth struct {
				Secret string `yaml:"secret"`
			} `yaml:"auth"`
		} `yaml:"http"`
	} `yaml:"delivery"`
	Event struct {
		Outbox struct {
			WorkerCount        int `yaml:"workerCount"`
			BatchSize          int `yaml:"batchSize"`
			PollIntervalMillis int `yaml:"pollIntervalMillis"`
			MaxAttempts        int `yaml:"maxAttempts"`
			BaseBackoffMillis  int `yaml:"baseBackoffMillis"`
		} `yaml:"outbox"`
	} `yaml:"event"`
	Jobs struct {
		Enabled           bool `yaml:"enabled"`
		AttachmentCleanup struct {
			Enabled            bool `yaml:"enabled"`
			IntervalSeconds    int  `yaml:"intervalSeconds"`
			GracePeriodSeconds int  `yaml:"gracePeriodSeconds"`
			BatchSize          int  `yaml:"batchSize"`
		} `yaml:"attachmentCleanup"`
	} `yaml:"jobs"`
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindEnv(v,
		"cache.listTTLSeconds",
		"cache.detailTTLSeconds",
		"admin.bootstrap.enabled",
		"admin.bootstrap.username",
		"admin.bootstrap.password",
		"storage.provider",
		"storage.local.rootDir",
		"storage.object.endpoint",
		"storage.object.bucket",
		"storage.object.accessKey",
		"storage.object.secretKey",
		"storage.object.useSSL",
		"storage.attachment.maxUploadSizeBytes",
		"storage.attachment.imageOptimization.enabled",
		"storage.attachment.imageOptimization.jpegQuality",
		"delivery.http.port",
		"delivery.http.maxJSONBodyBytes",
		"delivery.http.defaultPageLimit",
		"delivery.http.rateLimit.enabled",
		"delivery.http.rateLimit.windowSeconds",
		"delivery.http.rateLimit.writeRequests",
		"delivery.http.auth.secret",
		"event.outbox.workerCount",
		"event.outbox.batchSize",
		"event.outbox.pollIntervalMillis",
		"event.outbox.maxAttempts",
		"event.outbox.baseBackoffMillis",
		"jobs.enabled",
		"jobs.attachmentCleanup.enabled",
		"jobs.attachmentCleanup.intervalSeconds",
		"jobs.attachmentCleanup.gracePeriodSeconds",
		"jobs.attachmentCleanup.batchSize",
	)

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return nil, err
		}
	}

	return loadFromViper(v)
}

func bindEnv(v *viper.Viper, keys ...string) {
	for _, key := range keys {
		_ = v.BindEnv(key)
	}
}

func loadFromViper(v *viper.Viper) (*Config, error) {
	v.SetDefault("cache.listTTLSeconds", 30)
	v.SetDefault("cache.detailTTLSeconds", 30)
	v.SetDefault("admin.bootstrap.enabled", false)
	v.SetDefault("storage.provider", "local")
	v.SetDefault("storage.local.rootDir", "./data/uploads")
	v.SetDefault("storage.attachment.maxUploadSizeBytes", int64(10<<20))
	v.SetDefault("storage.attachment.imageOptimization.enabled", true)
	v.SetDefault("storage.attachment.imageOptimization.jpegQuality", 82)
	v.SetDefault("delivery.http.maxJSONBodyBytes", int64(1<<20))
	v.SetDefault("delivery.http.defaultPageLimit", 10)
	v.SetDefault("delivery.http.rateLimit.enabled", true)
	v.SetDefault("delivery.http.rateLimit.windowSeconds", 60)
	v.SetDefault("delivery.http.rateLimit.writeRequests", 60)
	v.SetDefault("event.outbox.workerCount", 1)
	v.SetDefault("event.outbox.batchSize", 100)
	v.SetDefault("event.outbox.pollIntervalMillis", 100)
	v.SetDefault("event.outbox.maxAttempts", 5)
	v.SetDefault("event.outbox.baseBackoffMillis", 200)
	v.SetDefault("jobs.enabled", true)
	v.SetDefault("jobs.attachmentCleanup.enabled", true)
	v.SetDefault("jobs.attachmentCleanup.intervalSeconds", 600)
	v.SetDefault("jobs.attachmentCleanup.gracePeriodSeconds", 600)
	v.SetDefault("jobs.attachmentCleanup.batchSize", 50)

	cfg := &Config{}
	if err := v.UnmarshalExact(cfg); err != nil {
		return nil, err
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validate(cfg *Config) error {
	port := cfg.Delivery.HTTP.Port
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid delivery.http.port: %d (must be 1..65535)", port)
	}
	if cfg.Delivery.HTTP.MaxJSONBodyBytes <= 0 {
		return fmt.Errorf("invalid delivery.http.maxJSONBodyBytes: %d (must be > 0)", cfg.Delivery.HTTP.MaxJSONBodyBytes)
	}
	if cfg.Delivery.HTTP.DefaultPageLimit < minDefaultPageLimit || cfg.Delivery.HTTP.DefaultPageLimit > maxDefaultPageLimit {
		return fmt.Errorf(
			"invalid delivery.http.defaultPageLimit: %d (must be %d..%d)",
			cfg.Delivery.HTTP.DefaultPageLimit,
			minDefaultPageLimit,
			maxDefaultPageLimit,
		)
	}
	if cfg.Delivery.HTTP.RateLimit.WindowSeconds < minRateLimitWindowSeconds {
		return fmt.Errorf(
			"invalid delivery.http.rateLimit.windowSeconds: %d (must be >= %d)",
			cfg.Delivery.HTTP.RateLimit.WindowSeconds,
			minRateLimitWindowSeconds,
		)
	}
	if cfg.Delivery.HTTP.RateLimit.WriteRequests < minRateLimitWriteRequests {
		return fmt.Errorf(
			"invalid delivery.http.rateLimit.writeRequests: %d (must be >= %d)",
			cfg.Delivery.HTTP.RateLimit.WriteRequests,
			minRateLimitWriteRequests,
		)
	}
	secret := strings.TrimSpace(cfg.Delivery.HTTP.Auth.Secret)
	if secret == "" {
		return fmt.Errorf("invalid delivery.http.auth.secret: cannot be empty")
	}
	if secret == placeholderJWTSecret {
		return fmt.Errorf("invalid delivery.http.auth.secret: placeholder secret is not allowed")
	}
	if len(secret) < minJWTSecretLength {
		return fmt.Errorf("invalid delivery.http.auth.secret: must be at least %d characters", minJWTSecretLength)
	}
	if cfg.Cache.ListTTLSeconds <= 0 {
		return fmt.Errorf("invalid cache.listTTLSeconds: %d (must be > 0)", cfg.Cache.ListTTLSeconds)
	}
	if cfg.Cache.DetailTTLSeconds <= 0 {
		return fmt.Errorf("invalid cache.detailTTLSeconds: %d (must be > 0)", cfg.Cache.DetailTTLSeconds)
	}
	switch cfg.Storage.Provider {
	case "local":
		if cfg.Storage.Local.RootDir == "" {
			return fmt.Errorf("invalid storage.local.rootDir: cannot be empty")
		}
	case "object":
		if cfg.Storage.Object.Endpoint == "" {
			return fmt.Errorf("invalid storage.object.endpoint: cannot be empty")
		}
		if cfg.Storage.Object.Bucket == "" {
			return fmt.Errorf("invalid storage.object.bucket: cannot be empty")
		}
		if cfg.Storage.Object.AccessKey == "" {
			return fmt.Errorf("invalid storage.object.accessKey: cannot be empty")
		}
		if cfg.Storage.Object.SecretKey == "" {
			return fmt.Errorf("invalid storage.object.secretKey: cannot be empty")
		}
	default:
		return fmt.Errorf("invalid storage.provider: %s", cfg.Storage.Provider)
	}
	if cfg.Storage.Attachment.MaxUploadSizeBytes <= 0 {
		return fmt.Errorf("invalid storage.attachment.maxUploadSizeBytes: %d (must be > 0)", cfg.Storage.Attachment.MaxUploadSizeBytes)
	}
	if cfg.Storage.Attachment.ImageOptimization.JPEGQuality < 1 || cfg.Storage.Attachment.ImageOptimization.JPEGQuality > 100 {
		return fmt.Errorf("invalid storage.attachment.imageOptimization.jpegQuality: %d (must be 1..100)", cfg.Storage.Attachment.ImageOptimization.JPEGQuality)
	}
	if cfg.Event.Outbox.WorkerCount <= 0 {
		return fmt.Errorf("invalid event.outbox.workerCount: %d (must be > 0)", cfg.Event.Outbox.WorkerCount)
	}
	if cfg.Event.Outbox.BatchSize <= 0 {
		return fmt.Errorf("invalid event.outbox.batchSize: %d (must be > 0)", cfg.Event.Outbox.BatchSize)
	}
	if cfg.Event.Outbox.PollIntervalMillis <= 0 {
		return fmt.Errorf("invalid event.outbox.pollIntervalMillis: %d (must be > 0)", cfg.Event.Outbox.PollIntervalMillis)
	}
	if cfg.Event.Outbox.MaxAttempts <= 0 {
		return fmt.Errorf("invalid event.outbox.maxAttempts: %d (must be > 0)", cfg.Event.Outbox.MaxAttempts)
	}
	if cfg.Event.Outbox.BaseBackoffMillis <= 0 {
		return fmt.Errorf("invalid event.outbox.baseBackoffMillis: %d (must be > 0)", cfg.Event.Outbox.BaseBackoffMillis)
	}
	if cfg.Jobs.Enabled && cfg.Jobs.AttachmentCleanup.Enabled {
		if cfg.Jobs.AttachmentCleanup.IntervalSeconds <= 0 {
			return fmt.Errorf("invalid jobs.attachmentCleanup.intervalSeconds: %d (must be > 0)", cfg.Jobs.AttachmentCleanup.IntervalSeconds)
		}
		if cfg.Jobs.AttachmentCleanup.GracePeriodSeconds <= 0 {
			return fmt.Errorf("invalid jobs.attachmentCleanup.gracePeriodSeconds: %d (must be > 0)", cfg.Jobs.AttachmentCleanup.GracePeriodSeconds)
		}
		if cfg.Jobs.AttachmentCleanup.BatchSize <= 0 {
			return fmt.Errorf("invalid jobs.attachmentCleanup.batchSize: %d (must be > 0)", cfg.Jobs.AttachmentCleanup.BatchSize)
		}
	}
	if cfg.Admin.Bootstrap.Enabled {
		if strings.TrimSpace(cfg.Admin.Bootstrap.Username) == "" {
			return fmt.Errorf("invalid admin.bootstrap.username: cannot be empty when bootstrap is enabled")
		}
		if strings.TrimSpace(cfg.Admin.Bootstrap.Password) == "" {
			return fmt.Errorf("invalid admin.bootstrap.password: cannot be empty when bootstrap is enabled")
		}
		if strings.TrimSpace(cfg.Admin.Bootstrap.Password) == placeholderBootstrapPassword {
			return fmt.Errorf("invalid admin.bootstrap.password: placeholder password is not allowed")
		}
	}
	return nil
}
