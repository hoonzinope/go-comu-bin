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
		Local struct {
			RootDir string `yaml:"rootDir"`
		} `yaml:"local"`
	} `yaml:"storage"`
	Delivery struct {
		HTTP struct {
			Port int `yaml:"port"`
			Auth struct {
				Secret string `yaml:"secret"`
			} `yaml:"auth"`
		} `yaml:"http"`
	} `yaml:"delivery"`
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
	v.SetDefault("storage.local.rootDir", "./data/uploads")

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
	if cfg.Storage.Local.RootDir == "" {
		return fmt.Errorf("invalid storage.local.rootDir: cannot be empty")
	}
	return nil
}
