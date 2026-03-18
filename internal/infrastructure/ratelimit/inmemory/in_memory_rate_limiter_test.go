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

func TestInMemoryRateLimiter_EvictsExpiredBuckets(t *testing.T) {
	limiter := NewInMemoryRateLimiter()
	ctx := context.Background()

	allowed, err := limiter.Allow(ctx, "signup:127.0.0.1", 1, time.Millisecond)
	require.NoError(t, err)
	assert.True(t, allowed)
	require.Len(t, limiter.buckets, 1)

	time.Sleep(5 * time.Millisecond)

	allowed, err = limiter.Allow(ctx, "signup:127.0.0.2", 1, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Len(t, limiter.buckets, 1)
	_, exists := limiter.buckets["signup:127.0.0.1"]
	assert.False(t, exists)
}
