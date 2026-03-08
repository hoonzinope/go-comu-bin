package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Cache struct {
		ListTTLSeconds   int `yaml:"listTTLSeconds"`
		DetailTTLSeconds int `yaml:"detailTTLSeconds"`
	} `yaml:"cache"`
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
			Port int `yaml:"port"`
			Auth struct {
				Secret string `yaml:"secret"`
			} `yaml:"auth"`
		} `yaml:"http"`
	} `yaml:"delivery"`
	Jobs struct {
		Enabled                 bool `yaml:"enabled"`
		OrphanAttachmentCleanup struct {
			Enabled            bool `yaml:"enabled"`
			IntervalSeconds    int  `yaml:"intervalSeconds"`
			GracePeriodSeconds int  `yaml:"gracePeriodSeconds"`
			BatchSize          int  `yaml:"batchSize"`
		} `yaml:"orphanAttachmentCleanup"`
	} `yaml:"jobs"`
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	return loadFromViper(v)
}

func loadFromViper(v *viper.Viper) (*Config, error) {
	v.SetDefault("cache.listTTLSeconds", 30)
	v.SetDefault("cache.detailTTLSeconds", 30)
	v.SetDefault("storage.provider", "local")
	v.SetDefault("storage.local.rootDir", "./data/uploads")
	v.SetDefault("storage.attachment.maxUploadSizeBytes", int64(10<<20))
	v.SetDefault("storage.attachment.imageOptimization.enabled", true)
	v.SetDefault("storage.attachment.imageOptimization.jpegQuality", 82)
	v.SetDefault("jobs.enabled", true)
	v.SetDefault("jobs.orphanAttachmentCleanup.enabled", true)
	v.SetDefault("jobs.orphanAttachmentCleanup.intervalSeconds", 600)
	v.SetDefault("jobs.orphanAttachmentCleanup.gracePeriodSeconds", 600)
	v.SetDefault("jobs.orphanAttachmentCleanup.batchSize", 50)

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
	if cfg.Delivery.HTTP.Auth.Secret == "" {
		return fmt.Errorf("invalid delivery.http.auth.secret: cannot be empty")
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
	if cfg.Jobs.OrphanAttachmentCleanup.IntervalSeconds <= 0 {
		return fmt.Errorf("invalid jobs.orphanAttachmentCleanup.intervalSeconds: %d (must be > 0)", cfg.Jobs.OrphanAttachmentCleanup.IntervalSeconds)
	}
	if cfg.Jobs.OrphanAttachmentCleanup.GracePeriodSeconds <= 0 {
		return fmt.Errorf("invalid jobs.orphanAttachmentCleanup.gracePeriodSeconds: %d (must be > 0)", cfg.Jobs.OrphanAttachmentCleanup.GracePeriodSeconds)
	}
	if cfg.Jobs.OrphanAttachmentCleanup.BatchSize <= 0 {
		return fmt.Errorf("invalid jobs.orphanAttachmentCleanup.batchSize: %d (must be > 0)", cfg.Jobs.OrphanAttachmentCleanup.BatchSize)
	}
	return nil
}
