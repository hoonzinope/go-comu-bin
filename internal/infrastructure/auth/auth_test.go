package auth

import (
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

func TestCacheSessionRepository(t *testing.T) {
	cache := cacheInMemory.NewInMemoryCache()
	repo := NewCacheSessionRepository(cache)

	require.NoError(t, repo.Save(7, "token-a", 1))
	exists, err := repo.Exists(7, "token-a")
	require.NoError(t, err)
	assert.True(t, exists)

	require.NoError(t, repo.Delete(7, "token-a"))
	exists, err = repo.Exists(7, "token-a")
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, repo.Save(7, "token-b", 1))
	require.NoError(t, repo.Save(7, "token-c", 1))
	require.NoError(t, repo.DeleteByUser(7))
	exists, err = repo.Exists(7, "token-b")
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, repo.Save(8, "token-d", 1))
	time.Sleep(1100 * time.Millisecond)
	exists, err = repo.Exists(8, "token-d")
	require.NoError(t, err)
	assert.False(t, exists)
}
