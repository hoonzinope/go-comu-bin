package porttest

import (
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func RunPostTagRepositoryContractTests(t *testing.T, newRepository func() port.PostTagRepository) {
	t.Helper()

	t.Run("upsert reactivates deleted relation", func(t *testing.T) {
		repo := newRepository()

		require.NoError(t, repo.UpsertActive(3, 7))
		require.NoError(t, repo.SoftDelete(3, 7))
		require.NoError(t, repo.UpsertActive(3, 7))

		items, err := repo.SelectActiveByPostID(3)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, int64(7), items[0].TagID)
	})

	t.Run("tag pagination sorts by post id desc", func(t *testing.T) {
		repo := newRepository()

		require.NoError(t, repo.UpsertActive(1, 5))
		require.NoError(t, repo.UpsertActive(2, 5))
		require.NoError(t, repo.UpsertActive(3, 5))

		items, err := repo.SelectActiveByTagID(5, 2, 0)
		require.NoError(t, err)
		require.Len(t, items, 2)
		assert.Equal(t, int64(3), items[0].PostID)
		assert.Equal(t, int64(2), items[1].PostID)
	})
}
