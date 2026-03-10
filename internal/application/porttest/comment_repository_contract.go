package porttest

import (
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func RunCommentRepositoryContractTests(t *testing.T, newRepository func() port.CommentRepository) {
	t.Helper()

	t.Run("select comments returns only active comments for the post in descending id order", func(t *testing.T) {
		repo := newRepository()

		active1 := entity.NewComment("active-1", 1, 10, nil)
		active1ID, err := repo.Save(active1)
		require.NoError(t, err)

		deleted := entity.NewComment("deleted", 1, 10, nil)
		deletedID, err := repo.Save(deleted)
		require.NoError(t, err)
		require.NoError(t, repo.Delete(deletedID))

		active2 := entity.NewComment("active-2", 1, 10, nil)
		active2ID, err := repo.Save(active2)
		require.NoError(t, err)

		otherPost := entity.NewComment("other-post", 1, 11, nil)
		_, err = repo.Save(otherPost)
		require.NoError(t, err)

		comments, err := repo.SelectComments(10, 10, 0)
		require.NoError(t, err)
		require.Len(t, comments, 2)
		assert.Equal(t, []int64{active2ID, active1ID}, []int64{comments[0].ID, comments[1].ID})
	})

	t.Run("select comments including deleted keeps tombstones for the same post", func(t *testing.T) {
		repo := newRepository()

		active := entity.NewComment("active", 1, 10, nil)
		activeID, err := repo.Save(active)
		require.NoError(t, err)

		tombstone := entity.NewComment("deleted", 1, 10, nil)
		tombstoneID, err := repo.Save(tombstone)
		require.NoError(t, err)
		require.NoError(t, repo.Delete(tombstoneID))

		otherPost := entity.NewComment("other-post", 1, 11, nil)
		_, err = repo.Save(otherPost)
		require.NoError(t, err)

		comments, err := repo.SelectCommentsIncludingDeleted(10)
		require.NoError(t, err)
		require.Len(t, comments, 2)
		assert.Equal(t, []int64{tombstoneID, activeID}, []int64{comments[0].ID, comments[1].ID})
		assert.Equal(t, entity.CommentStatusDeleted, comments[0].Status)
		assert.Equal(t, entity.DeletedCommentPlaceholder, comments[0].Content)
		assert.Equal(t, entity.CommentStatusActive, comments[1].Status)
	})

	t.Run("select comment by id excludes soft deleted comments", func(t *testing.T) {
		repo := newRepository()

		comment := entity.NewComment("hello", 1, 10, nil)
		id, err := repo.Save(comment)
		require.NoError(t, err)
		require.NoError(t, repo.Delete(id))

		selected, err := repo.SelectCommentByID(id)
		require.NoError(t, err)
		assert.Nil(t, selected)
	})

	t.Run("select comments applies cursor after filtering deleted comments", func(t *testing.T) {
		repo := newRepository()

		firstID, err := repo.Save(entity.NewComment("first", 1, 10, nil))
		require.NoError(t, err)
		secondID, err := repo.Save(entity.NewComment("second", 1, 10, nil))
		require.NoError(t, err)
		thirdID, err := repo.Save(entity.NewComment("third", 1, 10, nil))
		require.NoError(t, err)
		require.NoError(t, repo.Delete(secondID))

		comments, err := repo.SelectComments(10, 10, thirdID)
		require.NoError(t, err)
		require.Len(t, comments, 1)
		assert.Equal(t, firstID, comments[0].ID)
	})

	t.Run("select visible comments keeps deleted parent tombstone when active reply exists", func(t *testing.T) {
		repo := newRepository()

		parentID, err := repo.Save(entity.NewComment("parent", 1, 10, nil))
		require.NoError(t, err)
		require.NoError(t, repo.Delete(parentID))

		_, err = repo.Save(entity.NewComment("reply", 2, 10, &parentID))
		require.NoError(t, err)

		visible, err := repo.SelectVisibleComments(10, 10, 0)
		require.NoError(t, err)
		require.Len(t, visible, 2)
		assert.Equal(t, entity.CommentStatusActive, visible[0].Status)
		assert.Equal(t, parentID, visible[1].ID)
		assert.Equal(t, entity.CommentStatusDeleted, visible[1].Status)
	})
}
