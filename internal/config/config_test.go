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

	cfg, err := loadFromViper(v)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 18577, cfg.Delivery.HTTP.Port)
}

func TestLoadFromViper_InvalidPort(t *testing.T) {
	t.Run("port_is_zero", func(t *testing.T) {
		v := viper.New()
		v.Set("delivery.http.port", 0)
		v.Set("delivery.http.auth.secret", "test-secret")

		cfg, err := loadFromViper(v)
		require.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("port_is_out_of_range", func(t *testing.T) {
		v := viper.New()
		v.Set("delivery.http.port", 70000)
		v.Set("delivery.http.auth.secret", "test-secret")

		cfg, err := loadFromViper(v)
		require.Error(t, err)
		assert.Nil(t, cfg)
	})
}

func TestLoadFromViper_UnknownField(t *testing.T) {
	v := viper.New()
	v.Set("delivery.http.port", 18577)
	v.Set("delivery.http.auth.secret", "test-secret")
	v.Set("delivery.http.unknown", true)

	cfg, err := loadFromViper(v)
	require.Error(t, err)
	assert.Nil(t, cfg)
}
