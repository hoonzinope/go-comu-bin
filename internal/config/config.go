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
const minRateLimitReadRequests = 1
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
				ReadRequests  int  `yaml:"readRequests"`
				WriteRequests int  `yaml:"writeRequests"`
			} `yaml:"rateLimit"`
			Auth struct {
				Secret         string `yaml:"secret"`
				LoginRateLimit struct {
					Enabled       bool `yaml:"enabled"`
					WindowSeconds int  `yaml:"windowSeconds"`
					MaxRequests   int  `yaml:"maxRequests"`
				} `yaml:"loginRateLimit"`
				GuestUpgradeRateLimit struct {
					Enabled       bool `yaml:"enabled"`
					WindowSeconds int  `yaml:"windowSeconds"`
					MaxRequests   int  `yaml:"maxRequests"`
				} `yaml:"guestUpgradeRateLimit"`
				EmailVerificationRequestRateLimit struct {
					Enabled       bool `yaml:"enabled"`
					WindowSeconds int  `yaml:"windowSeconds"`
					MaxRequests   int  `yaml:"maxRequests"`
				} `yaml:"emailVerificationRequestRateLimit"`
				PasswordResetRequestRateLimit struct {
					Enabled       bool `yaml:"enabled"`
					WindowSeconds int  `yaml:"windowSeconds"`
					MaxRequests   int  `yaml:"maxRequests"`
				} `yaml:"passwordResetRequestRateLimit"`
			} `yaml:"auth"`
		} `yaml:"http"`
		Mail struct {
			Enabled           bool `yaml:"enabled"`
			EmailVerification struct {
				BaseURL string `yaml:"baseURL"`
			} `yaml:"emailVerification"`
			PasswordReset struct {
				BaseURL string `yaml:"baseURL"`
			} `yaml:"passwordReset"`
			SMTP struct {
				Host        string `yaml:"host"`
				Port        int    `yaml:"port"`
				Username    string `yaml:"username"`
				Password    string `yaml:"password"`
				From        string `yaml:"from"`
				StartTLS    bool   `yaml:"startTLS"`
				ImplicitTLS bool   `yaml:"implicitTLS"`
			} `yaml:"smtp"`
		} `yaml:"mail"`
	} `yaml:"delivery"`
	Event struct {
		Outbox struct {
			WorkerCount           int `yaml:"workerCount"`
			BatchSize             int `yaml:"batchSize"`
			PollIntervalMillis    int `yaml:"pollIntervalMillis"`
			MaxAttempts           int `yaml:"maxAttempts"`
			BaseBackoffMillis     int `yaml:"baseBackoffMillis"`
			ProcessingLeaseMillis int `yaml:"processingLeaseMillis"`
			LeaseRefreshMillis    int `yaml:"leaseRefreshMillis"`
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
		GuestCleanup struct {
			Enabled                        bool `yaml:"enabled"`
			IntervalSeconds                int  `yaml:"intervalSeconds"`
			PendingGracePeriodSeconds      int  `yaml:"pendingGracePeriodSeconds"`
			ActiveUnusedGracePeriodSeconds int  `yaml:"activeUnusedGracePeriodSeconds"`
			BatchSize                      int  `yaml:"batchSize"`
		} `yaml:"guestCleanup"`
		EmailVerificationCleanup struct {
			Enabled            bool `yaml:"enabled"`
			IntervalSeconds    int  `yaml:"intervalSeconds"`
			GracePeriodSeconds int  `yaml:"gracePeriodSeconds"`
			BatchSize          int  `yaml:"batchSize"`
		} `yaml:"emailVerificationCleanup"`
		PasswordResetCleanup struct {
			Enabled            bool `yaml:"enabled"`
			IntervalSeconds    int  `yaml:"intervalSeconds"`
			GracePeriodSeconds int  `yaml:"gracePeriodSeconds"`
			BatchSize          int  `yaml:"batchSize"`
		} `yaml:"passwordResetCleanup"`
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
		"delivery.http.rateLimit.readRequests",
		"delivery.http.rateLimit.writeRequests",
		"delivery.http.auth.secret",
		"delivery.http.auth.loginRateLimit.enabled",
		"delivery.http.auth.loginRateLimit.windowSeconds",
		"delivery.http.auth.loginRateLimit.maxRequests",
		"delivery.http.auth.guestUpgradeRateLimit.enabled",
		"delivery.http.auth.guestUpgradeRateLimit.windowSeconds",
		"delivery.http.auth.guestUpgradeRateLimit.maxRequests",
		"delivery.http.auth.emailVerificationRequestRateLimit.enabled",
		"delivery.http.auth.emailVerificationRequestRateLimit.windowSeconds",
		"delivery.http.auth.emailVerificationRequestRateLimit.maxRequests",
		"delivery.http.auth.passwordResetRequestRateLimit.enabled",
		"delivery.http.auth.passwordResetRequestRateLimit.windowSeconds",
		"delivery.http.auth.passwordResetRequestRateLimit.maxRequests",
		"delivery.mail.enabled",
		"delivery.mail.emailVerification.baseURL",
		"delivery.mail.passwordReset.baseURL",
		"delivery.mail.smtp.host",
		"delivery.mail.smtp.port",
		"delivery.mail.smtp.username",
		"delivery.mail.smtp.password",
		"delivery.mail.smtp.from",
		"delivery.mail.smtp.startTLS",
		"delivery.mail.smtp.implicitTLS",
		"event.outbox.workerCount",
		"event.outbox.batchSize",
		"event.outbox.pollIntervalMillis",
		"event.outbox.maxAttempts",
		"event.outbox.baseBackoffMillis",
		"event.outbox.processingLeaseMillis",
		"event.outbox.leaseRefreshMillis",
		"jobs.enabled",
		"jobs.attachmentCleanup.enabled",
		"jobs.attachmentCleanup.intervalSeconds",
		"jobs.attachmentCleanup.gracePeriodSeconds",
		"jobs.attachmentCleanup.batchSize",
		"jobs.guestCleanup.enabled",
		"jobs.guestCleanup.intervalSeconds",
		"jobs.guestCleanup.pendingGracePeriodSeconds",
		"jobs.guestCleanup.activeUnusedGracePeriodSeconds",
		"jobs.guestCleanup.batchSize",
		"jobs.emailVerificationCleanup.enabled",
		"jobs.emailVerificationCleanup.intervalSeconds",
		"jobs.emailVerificationCleanup.gracePeriodSeconds",
		"jobs.emailVerificationCleanup.batchSize",
		"jobs.passwordResetCleanup.enabled",
		"jobs.passwordResetCleanup.intervalSeconds",
		"jobs.passwordResetCleanup.gracePeriodSeconds",
		"jobs.passwordResetCleanup.batchSize",
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
	v.SetDefault("delivery.http.rateLimit.readRequests", 300)
	v.SetDefault("delivery.http.rateLimit.writeRequests", 60)
	v.SetDefault("delivery.http.auth.loginRateLimit.enabled", true)
	v.SetDefault("delivery.http.auth.loginRateLimit.windowSeconds", 60)
	v.SetDefault("delivery.http.auth.loginRateLimit.maxRequests", 5)
	v.SetDefault("delivery.http.auth.guestUpgradeRateLimit.enabled", true)
	v.SetDefault("delivery.http.auth.guestUpgradeRateLimit.windowSeconds", 60)
	v.SetDefault("delivery.http.auth.guestUpgradeRateLimit.maxRequests", 5)
	v.SetDefault("delivery.http.auth.emailVerificationRequestRateLimit.enabled", true)
	v.SetDefault("delivery.http.auth.emailVerificationRequestRateLimit.windowSeconds", 60)
	v.SetDefault("delivery.http.auth.emailVerificationRequestRateLimit.maxRequests", 5)
	v.SetDefault("delivery.http.auth.passwordResetRequestRateLimit.enabled", true)
	v.SetDefault("delivery.http.auth.passwordResetRequestRateLimit.windowSeconds", 60)
	v.SetDefault("delivery.http.auth.passwordResetRequestRateLimit.maxRequests", 5)
	v.SetDefault("delivery.mail.enabled", false)
	v.SetDefault("delivery.mail.smtp.port", 587)
	v.SetDefault("event.outbox.workerCount", 1)
	v.SetDefault("event.outbox.batchSize", 100)
	v.SetDefault("event.outbox.pollIntervalMillis", 100)
	v.SetDefault("event.outbox.maxAttempts", 5)
	v.SetDefault("event.outbox.baseBackoffMillis", 200)
	v.SetDefault("event.outbox.processingLeaseMillis", 30000)
	v.SetDefault("event.outbox.leaseRefreshMillis", 10000)
	v.SetDefault("jobs.enabled", true)
	v.SetDefault("jobs.attachmentCleanup.enabled", true)
	v.SetDefault("jobs.attachmentCleanup.intervalSeconds", 600)
	v.SetDefault("jobs.attachmentCleanup.gracePeriodSeconds", 600)
	v.SetDefault("jobs.attachmentCleanup.batchSize", 50)
	v.SetDefault("jobs.guestCleanup.enabled", true)
	v.SetDefault("jobs.guestCleanup.intervalSeconds", 600)
	v.SetDefault("jobs.guestCleanup.pendingGracePeriodSeconds", 600)
	v.SetDefault("jobs.guestCleanup.activeUnusedGracePeriodSeconds", 86400)
	v.SetDefault("jobs.guestCleanup.batchSize", 50)
	v.SetDefault("jobs.emailVerificationCleanup.enabled", true)
	v.SetDefault("jobs.emailVerificationCleanup.intervalSeconds", 600)
	v.SetDefault("jobs.emailVerificationCleanup.gracePeriodSeconds", 600)
	v.SetDefault("jobs.emailVerificationCleanup.batchSize", 100)
	v.SetDefault("jobs.passwordResetCleanup.enabled", true)
	v.SetDefault("jobs.passwordResetCleanup.intervalSeconds", 600)
	v.SetDefault("jobs.passwordResetCleanup.gracePeriodSeconds", 600)
	v.SetDefault("jobs.passwordResetCleanup.batchSize", 100)

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
	if cfg.Delivery.HTTP.RateLimit.ReadRequests < minRateLimitReadRequests {
		return fmt.Errorf(
			"invalid delivery.http.rateLimit.readRequests: %d (must be >= %d)",
			cfg.Delivery.HTTP.RateLimit.ReadRequests,
			minRateLimitReadRequests,
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
	if cfg.Delivery.HTTP.Auth.LoginRateLimit.Enabled {
		if cfg.Delivery.HTTP.Auth.LoginRateLimit.WindowSeconds < minRateLimitWindowSeconds {
			return fmt.Errorf(
				"invalid delivery.http.auth.loginRateLimit.windowSeconds: %d (must be >= %d)",
				cfg.Delivery.HTTP.Auth.LoginRateLimit.WindowSeconds,
				minRateLimitWindowSeconds,
			)
		}
		if cfg.Delivery.HTTP.Auth.LoginRateLimit.MaxRequests < minRateLimitWriteRequests {
			return fmt.Errorf(
				"invalid delivery.http.auth.loginRateLimit.maxRequests: %d (must be >= %d)",
				cfg.Delivery.HTTP.Auth.LoginRateLimit.MaxRequests,
				minRateLimitWriteRequests,
			)
		}
	}
	if cfg.Delivery.HTTP.Auth.GuestUpgradeRateLimit.Enabled {
		if cfg.Delivery.HTTP.Auth.GuestUpgradeRateLimit.WindowSeconds < minRateLimitWindowSeconds {
			return fmt.Errorf(
				"invalid delivery.http.auth.guestUpgradeRateLimit.windowSeconds: %d (must be >= %d)",
				cfg.Delivery.HTTP.Auth.GuestUpgradeRateLimit.WindowSeconds,
				minRateLimitWindowSeconds,
			)
		}
		if cfg.Delivery.HTTP.Auth.GuestUpgradeRateLimit.MaxRequests < minRateLimitWriteRequests {
			return fmt.Errorf(
				"invalid delivery.http.auth.guestUpgradeRateLimit.maxRequests: %d (must be >= %d)",
				cfg.Delivery.HTTP.Auth.GuestUpgradeRateLimit.MaxRequests,
				minRateLimitWriteRequests,
			)
		}
	}
	if cfg.Delivery.HTTP.Auth.EmailVerificationRequestRateLimit.Enabled {
		if cfg.Delivery.HTTP.Auth.EmailVerificationRequestRateLimit.WindowSeconds < minRateLimitWindowSeconds {
			return fmt.Errorf(
				"invalid delivery.http.auth.emailVerificationRequestRateLimit.windowSeconds: %d (must be >= %d)",
				cfg.Delivery.HTTP.Auth.EmailVerificationRequestRateLimit.WindowSeconds,
				minRateLimitWindowSeconds,
			)
		}
		if cfg.Delivery.HTTP.Auth.EmailVerificationRequestRateLimit.MaxRequests < minRateLimitWriteRequests {
			return fmt.Errorf(
				"invalid delivery.http.auth.emailVerificationRequestRateLimit.maxRequests: %d (must be >= %d)",
				cfg.Delivery.HTTP.Auth.EmailVerificationRequestRateLimit.MaxRequests,
				minRateLimitWriteRequests,
			)
		}
	}
	if cfg.Delivery.HTTP.Auth.PasswordResetRequestRateLimit.Enabled {
		if cfg.Delivery.HTTP.Auth.PasswordResetRequestRateLimit.WindowSeconds < minRateLimitWindowSeconds {
			return fmt.Errorf(
				"invalid delivery.http.auth.passwordResetRequestRateLimit.windowSeconds: %d (must be >= %d)",
				cfg.Delivery.HTTP.Auth.PasswordResetRequestRateLimit.WindowSeconds,
				minRateLimitWindowSeconds,
			)
		}
		if cfg.Delivery.HTTP.Auth.PasswordResetRequestRateLimit.MaxRequests < minRateLimitWriteRequests {
			return fmt.Errorf(
				"invalid delivery.http.auth.passwordResetRequestRateLimit.maxRequests: %d (must be >= %d)",
				cfg.Delivery.HTTP.Auth.PasswordResetRequestRateLimit.MaxRequests,
				minRateLimitWriteRequests,
			)
		}
	}
	if cfg.Delivery.Mail.Enabled {
		if strings.TrimSpace(cfg.Delivery.Mail.EmailVerification.BaseURL) == "" {
			return fmt.Errorf("invalid delivery.mail.emailVerification.baseURL: cannot be empty when mail is enabled")
		}
		if strings.TrimSpace(cfg.Delivery.Mail.PasswordReset.BaseURL) == "" {
			return fmt.Errorf("invalid delivery.mail.passwordReset.baseURL: cannot be empty when mail is enabled")
		}
		if strings.TrimSpace(cfg.Delivery.Mail.SMTP.Host) == "" {
			return fmt.Errorf("invalid delivery.mail.smtp.host: cannot be empty when mail is enabled")
		}
		if cfg.Delivery.Mail.SMTP.Port < 1 || cfg.Delivery.Mail.SMTP.Port > 65535 {
			return fmt.Errorf("invalid delivery.mail.smtp.port: %d (must be 1..65535)", cfg.Delivery.Mail.SMTP.Port)
		}
		if strings.TrimSpace(cfg.Delivery.Mail.SMTP.From) == "" {
			return fmt.Errorf("invalid delivery.mail.smtp.from: cannot be empty when mail is enabled")
		}
		if cfg.Delivery.Mail.SMTP.StartTLS && cfg.Delivery.Mail.SMTP.ImplicitTLS {
			return fmt.Errorf("invalid delivery.mail.smtp: startTLS and implicitTLS cannot both be enabled")
		}
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
	if cfg.Event.Outbox.ProcessingLeaseMillis <= 0 {
		return fmt.Errorf("invalid event.outbox.processingLeaseMillis: %d (must be > 0)", cfg.Event.Outbox.ProcessingLeaseMillis)
	}
	if cfg.Event.Outbox.LeaseRefreshMillis <= 0 {
		return fmt.Errorf("invalid event.outbox.leaseRefreshMillis: %d (must be > 0)", cfg.Event.Outbox.LeaseRefreshMillis)
	}
	if cfg.Event.Outbox.LeaseRefreshMillis >= cfg.Event.Outbox.ProcessingLeaseMillis {
		return fmt.Errorf(
			"invalid event.outbox.leaseRefreshMillis: %d (must be < processingLeaseMillis %d)",
			cfg.Event.Outbox.LeaseRefreshMillis,
			cfg.Event.Outbox.ProcessingLeaseMillis,
		)
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
	if cfg.Jobs.Enabled && cfg.Jobs.GuestCleanup.Enabled {
		if cfg.Jobs.GuestCleanup.IntervalSeconds <= 0 {
			return fmt.Errorf("invalid jobs.guestCleanup.intervalSeconds: %d (must be > 0)", cfg.Jobs.GuestCleanup.IntervalSeconds)
		}
		if cfg.Jobs.GuestCleanup.PendingGracePeriodSeconds <= 0 {
			return fmt.Errorf("invalid jobs.guestCleanup.pendingGracePeriodSeconds: %d (must be > 0)", cfg.Jobs.GuestCleanup.PendingGracePeriodSeconds)
		}
		if cfg.Jobs.GuestCleanup.ActiveUnusedGracePeriodSeconds <= 0 {
			return fmt.Errorf("invalid jobs.guestCleanup.activeUnusedGracePeriodSeconds: %d (must be > 0)", cfg.Jobs.GuestCleanup.ActiveUnusedGracePeriodSeconds)
		}
		if cfg.Jobs.GuestCleanup.BatchSize <= 0 {
			return fmt.Errorf("invalid jobs.guestCleanup.batchSize: %d (must be > 0)", cfg.Jobs.GuestCleanup.BatchSize)
		}
	}
	if cfg.Jobs.Enabled && cfg.Jobs.EmailVerificationCleanup.Enabled {
		if cfg.Jobs.EmailVerificationCleanup.IntervalSeconds <= 0 {
			return fmt.Errorf("invalid jobs.emailVerificationCleanup.intervalSeconds: %d (must be > 0)", cfg.Jobs.EmailVerificationCleanup.IntervalSeconds)
		}
		if cfg.Jobs.EmailVerificationCleanup.GracePeriodSeconds <= 0 {
			return fmt.Errorf("invalid jobs.emailVerificationCleanup.gracePeriodSeconds: %d (must be > 0)", cfg.Jobs.EmailVerificationCleanup.GracePeriodSeconds)
		}
		if cfg.Jobs.EmailVerificationCleanup.BatchSize <= 0 {
			return fmt.Errorf("invalid jobs.emailVerificationCleanup.batchSize: %d (must be > 0)", cfg.Jobs.EmailVerificationCleanup.BatchSize)
		}
	}
	if cfg.Jobs.Enabled && cfg.Jobs.PasswordResetCleanup.Enabled {
		if cfg.Jobs.PasswordResetCleanup.IntervalSeconds <= 0 {
			return fmt.Errorf("invalid jobs.passwordResetCleanup.intervalSeconds: %d (must be > 0)", cfg.Jobs.PasswordResetCleanup.IntervalSeconds)
		}
		if cfg.Jobs.PasswordResetCleanup.GracePeriodSeconds <= 0 {
			return fmt.Errorf("invalid jobs.passwordResetCleanup.gracePeriodSeconds: %d (must be > 0)", cfg.Jobs.PasswordResetCleanup.GracePeriodSeconds)
		}
		if cfg.Jobs.PasswordResetCleanup.BatchSize <= 0 {
			return fmt.Errorf("invalid jobs.passwordResetCleanup.batchSize: %d (must be > 0)", cfg.Jobs.PasswordResetCleanup.BatchSize)
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
