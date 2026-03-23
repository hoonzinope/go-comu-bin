package inmemory

import (
	"context"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordResetTokenRepository_SaveSelectInvalidateAndUpdate(t *testing.T) {
	repo := NewPasswordResetTokenRepository()
	token := entity.NewPasswordResetToken(1, "hash-1", time.Now().Add(time.Hour))

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

	next := entity.NewPasswordResetToken(1, "hash-2", time.Now().Add(time.Hour))
	require.NoError(t, repo.Save(context.Background(), next))
	next.Consume(time.Now())
	require.NoError(t, repo.Update(context.Background(), next))

	updated, err := repo.SelectByTokenHash(context.Background(), "hash-2")
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.True(t, updated.IsConsumed())
}
