package inmemory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryRateLimiter_Allow(t *testing.T) {
	limiter := NewInMemoryRateLimiter()
	ctx := context.Background()

	allowed, err := limiter.Allow(ctx, "signup:127.0.0.1", 2, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = limiter.Allow(ctx, "signup:127.0.0.1", 2, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = limiter.Allow(ctx, "signup:127.0.0.1", 2, time.Minute)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestInMemoryRateLimiter_SeparateKeys(t *testing.T) {
	limiter := NewInMemoryRateLimiter()
	ctx := context.Background()

	allowed, err := limiter.Allow(ctx, "signup:127.0.0.1", 1, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = limiter.Allow(ctx, "signup:127.0.0.2", 1, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed)
}
