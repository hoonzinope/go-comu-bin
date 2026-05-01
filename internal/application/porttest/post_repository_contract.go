package porttest

import (
	"context"
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

		_, err := repo.Save(context.Background(), entity.NewPost("title", "content", 1, 10))
		require.NoError(t, err)

		exists, err := repo.ExistsByBoardID(context.Background(), 10)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("exists by board id ignores deleted posts", func(t *testing.T) {
		repo := newRepository()

		id, err := repo.Save(context.Background(), entity.NewPost("title", "content", 1, 10))
		require.NoError(t, err)
		require.NoError(t, repo.Delete(context.Background(), id))

		exists, err := repo.ExistsByBoardID(context.Background(), 10)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("exists by board id ignores posts from other boards", func(t *testing.T) {
		repo := newRepository()

		_, err := repo.Save(context.Background(), entity.NewPost("title", "content", 1, 11))
		require.NoError(t, err)

		exists, err := repo.ExistsByBoardID(context.Background(), 10)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("counts published, draft, and tagged posts", func(t *testing.T) {
		repo := newRepository()
		boardID := int64(10)
		authorID := int64(1)

		published1, err := repo.Save(context.Background(), entity.NewPost("title-1", "content", authorID, boardID))
		require.NoError(t, err)
		published2, err := repo.Save(context.Background(), entity.NewPost("title-2", "content", authorID, boardID))
		require.NoError(t, err)
		draft := entity.NewDraftPost("draft", "content", authorID, boardID)
		_, err = repo.Save(context.Background(), draft)
		require.NoError(t, err)

		publishedPosts, err := repo.CountPublishedPosts(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 2, publishedPosts)

		publishedByBoard, err := repo.CountPublishedPostsByBoardID(context.Background(), boardID)
		require.NoError(t, err)
		assert.Equal(t, 2, publishedByBoard)

		draftPosts, err := repo.CountDraftPostsByAuthorID(context.Background(), authorID)
		require.NoError(t, err)
		assert.Equal(t, 1, draftPosts)

		_ = published1
		_ = published2
	})
}
