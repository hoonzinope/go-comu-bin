package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Delivery struct {
		HTTP struct {
			Port int `yaml:"port"`
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
	return nil
}
