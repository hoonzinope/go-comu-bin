package inmemory

import (
	"context"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailVerificationTokenRepository_SaveSelectInvalidateAndUpdate(t *testing.T) {
	repo := NewEmailVerificationTokenRepository()
	token := entity.NewEmailVerificationToken(1, "hash-1", time.Now().Add(time.Hour))

	require.NoError(t, repo.Save(context.Background(), token))

	saved, err := repo.SelectByTokenHash(context.Background(), "hash-1")
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, int64(1), saved.UserID)
	assert.True(t, saved.IsUsable(time.Now()))

	require.NoError(t, repo.InvalidateByUser(context.Background(), 1))

	invalidated, err := repo.SelectByTokenHash(context.Background(), "hash-1")
	require.NoError(t, err)
	require.NotNil(t, invalidated)
	assert.True(t, invalidated.IsConsumed())

	next := entity.NewEmailVerificationToken(1, "hash-2", time.Now().Add(time.Hour))
	require.NoError(t, repo.Save(context.Background(), next))
	next.Consume(time.Now())
	require.NoError(t, repo.Update(context.Background(), next))

	updated, err := repo.SelectByTokenHash(context.Background(), "hash-2")
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.True(t, updated.IsConsumed())
}

func TestEmailVerificationTokenRepository_DeleteExpiredOrConsumedBefore(t *testing.T) {
	repo := NewEmailVerificationTokenRepository()
	expired := entity.NewEmailVerificationToken(1, "expired", time.Now().Add(-2*time.Hour))
	consumed := entity.NewEmailVerificationToken(1, "consumed", time.Now().Add(time.Hour))
	consumedAt := time.Now().Add(-2 * time.Hour)
	consumed.ConsumedAt = &consumedAt
	fresh := entity.NewEmailVerificationToken(1, "fresh", time.Now().Add(time.Hour))

	require.NoError(t, repo.Save(context.Background(), expired))
	require.NoError(t, repo.Save(context.Background(), consumed))
	require.NoError(t, repo.Save(context.Background(), fresh))

	deleted, err := repo.DeleteExpiredOrConsumedBefore(context.Background(), time.Now().Add(-time.Hour), 10)
	require.NoError(t, err)
	assert.Equal(t, 2, deleted)

	item, err := repo.SelectByTokenHash(context.Background(), "expired")
	require.NoError(t, err)
	assert.Nil(t, item)

	item, err = repo.SelectByTokenHash(context.Background(), "consumed")
	require.NoError(t, err)
	assert.Nil(t, item)

	item, err = repo.SelectByTokenHash(context.Background(), "fresh")
	require.NoError(t, err)
	require.NotNil(t, item)
}

func TestEmailVerificationTokenRepository_DeleteExpiredOrConsumedBefore_RespectsLimit(t *testing.T) {
	repo := NewEmailVerificationTokenRepository()
	for i := 0; i < 3; i++ {
		token := entity.NewEmailVerificationToken(1, string(rune('a'+i)), time.Now().Add(-2*time.Hour))
		require.NoError(t, repo.Save(context.Background(), token))
	}

	deleted, err := repo.DeleteExpiredOrConsumedBefore(context.Background(), time.Now().Add(-time.Hour), 2)
	require.NoError(t, err)
	assert.Equal(t, 2, deleted)
}
