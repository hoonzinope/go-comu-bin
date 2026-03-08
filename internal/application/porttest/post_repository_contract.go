package porttest

import (
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func RunPostRepositoryContractTests(t *testing.T, newRepository func() port.PostRepository) {
	t.Helper()

	t.Run("exists by board id returns true for active posts", func(t *testing.T) {
		repo := newRepository()

		_, err := repo.Save(entity.NewPost("title", "content", 1, 10))
		require.NoError(t, err)

		exists, err := repo.ExistsByBoardID(10)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("exists by board id ignores deleted posts", func(t *testing.T) {
		repo := newRepository()

		id, err := repo.Save(entity.NewPost("title", "content", 1, 10))
		require.NoError(t, err)
		require.NoError(t, repo.Delete(id))

		exists, err := repo.ExistsByBoardID(10)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("exists by board id ignores posts from other boards", func(t *testing.T) {
		repo := newRepository()

		_, err := repo.Save(entity.NewPost("title", "content", 1, 11))
		require.NoError(t, err)

		exists, err := repo.ExistsByBoardID(10)
		require.NoError(t, err)
		assert.False(t, exists)
	})
}
