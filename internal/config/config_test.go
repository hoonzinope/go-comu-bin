package config

import (
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

	cfg, err := loadFromViper(v)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 18577, cfg.Delivery.HTTP.Port)
	assert.Equal(t, 30, cfg.Cache.ListTTLSeconds)
	assert.Equal(t, 60, cfg.Cache.DetailTTLSeconds)
}

func TestLoadFromViper_InvalidPort(t *testing.T) {
	t.Run("port_is_zero", func(t *testing.T) {
		v := viper.New()
		v.Set("delivery.http.port", 0)
		v.Set("delivery.http.auth.secret", "test-secret")
		v.Set("cache.listTTLSeconds", 30)
		v.Set("cache.detailTTLSeconds", 30)

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

	cfg, err := loadFromViper(v)
	require.Error(t, err)
	assert.Nil(t, cfg)
}
