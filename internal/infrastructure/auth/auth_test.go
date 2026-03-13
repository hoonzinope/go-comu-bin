package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	cacheInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestBcryptPasswordHasher(t *testing.T) {
	hasher := NewBcryptPasswordHasher(4)

	hashed, err := hasher.Hash("pw")
	require.NoError(t, err)
	assert.NotEmpty(t, hashed)

	matched, err := hasher.Matches(hashed, "pw")
	require.NoError(t, err)
	assert.True(t, matched)

	matched, err = hasher.Matches(hashed, "wrong")
	require.NoError(t, err)
	assert.False(t, matched)

	_, err = NewBcryptPasswordHasher(4).Matches("invalid-hash", "pw")
	require.Error(t, err)
	assert.False(t, errors.Is(err, bcrypt.ErrMismatchedHashAndPassword))
}

func TestJwtTokenProvider(t *testing.T) {
	provider := NewJwtTokenProvider("secret")

	token, err := provider.IdToToken(7)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, 24*60*60, provider.TTLSeconds())

	userID, err := provider.ValidateTokenToId(token)
	require.NoError(t, err)
	assert.Equal(t, int64(7), userID)

	_, err = provider.ValidateTokenToId("invalid")
	require.Error(t, err)
}

func TestJwtTokenProvider_LargeUserIDRoundTrip(t *testing.T) {
	provider := NewJwtTokenProvider("secret")
	// 2^53+1: float64 기반 claim 파싱이면 정밀도 손실이 발생할 수 있다.
	userID := int64(9007199254740993)

	token, err := provider.IdToToken(userID)
	require.NoError(t, err)

	got, err := provider.ValidateTokenToId(token)
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestCacheSessionRepository(t *testing.T) {
	cache := cacheInMemory.NewInMemoryCache()
	repo := NewCacheSessionRepository(cache)
	ctx := context.Background()

	require.NoError(t, repo.Save(ctx, 7, "token-a", 1))
	exists, err := repo.Exists(ctx, 7, "token-a")
	require.NoError(t, err)
	assert.True(t, exists)

	require.NoError(t, repo.Delete(ctx, 7, "token-a"))
	exists, err = repo.Exists(ctx, 7, "token-a")
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, repo.Save(ctx, 7, "token-b", 1))
	require.NoError(t, repo.Save(ctx, 7, "token-c", 1))
	require.NoError(t, repo.DeleteByUser(ctx, 7))
	exists, err = repo.Exists(ctx, 7, "token-b")
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, repo.Save(ctx, 8, "token-d", 1))
	time.Sleep(1100 * time.Millisecond)
	exists, err = repo.Exists(ctx, 8, "token-d")
	require.NoError(t, err)
	assert.False(t, exists)
}
