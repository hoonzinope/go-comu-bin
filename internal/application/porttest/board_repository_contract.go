package porttest

import (
	"context"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func RunBoardRepositoryContractTests(t *testing.T, newRepository func() port.BoardRepository) {
	t.Helper()

	t.Run("save select update delete", func(t *testing.T) {
		repo := newRepository()

		id, err := repo.Save(context.Background(), entity.NewBoard("free", "desc"))
		require.NoError(t, err)

		selected, err := repo.SelectBoardByID(context.Background(), id)
		require.NoError(t, err)
		require.NotNil(t, selected)
		assert.Equal(t, "free", selected.Name)

		selected.Update("notice", "updated")
		require.NoError(t, repo.Update(context.Background(), selected))

		updated, err := repo.SelectBoardByID(context.Background(), id)
		require.NoError(t, err)
		require.NotNil(t, updated)
		assert.Equal(t, "notice", updated.Name)

		require.NoError(t, repo.Delete(context.Background(), id))

		deleted, err := repo.SelectBoardByID(context.Background(), id)
		require.NoError(t, err)
		assert.Nil(t, deleted)
	})

	t.Run("list uses descending id order", func(t *testing.T) {
		repo := newRepository()
		_, _ = repo.Save(context.Background(), entity.NewBoard("b1", "d1"))
		_, _ = repo.Save(context.Background(), entity.NewBoard("b2", "d2"))
		_, _ = repo.Save(context.Background(), entity.NewBoard("b3", "d3"))

		boards, err := repo.SelectBoardList(context.Background(), 3, 0)
		require.NoError(t, err)
		require.Len(t, boards, 3)
		assert.Equal(t, int64(3), boards[0].ID)
		assert.Equal(t, int64(2), boards[1].ID)
		assert.Equal(t, int64(1), boards[2].ID)
	})

	t.Run("last id is exclusive cursor", func(t *testing.T) {
		repo := newRepository()
		_, _ = repo.Save(context.Background(), entity.NewBoard("b1", "d1"))
		_, _ = repo.Save(context.Background(), entity.NewBoard("b2", "d2"))
		_, _ = repo.Save(context.Background(), entity.NewBoard("b3", "d3"))

		boards, err := repo.SelectBoardList(context.Background(), 10, 3)
		require.NoError(t, err)
		require.Len(t, boards, 2)
		assert.Equal(t, int64(2), boards[0].ID)
		assert.Equal(t, int64(1), boards[1].ID)
	})

	t.Run("non positive limit returns empty", func(t *testing.T) {
		repo := newRepository()
		_, _ = repo.Save(context.Background(), entity.NewBoard("b1", "d1"))

		boards, err := repo.SelectBoardList(context.Background(), 0, 0)
		require.NoError(t, err)
		assert.Empty(t, boards)
	})

	t.Run("update and delete missing id are no-op", func(t *testing.T) {
		repo := newRepository()

		board := entity.NewBoard("free", "desc")
		board.ID = 999

		require.NoError(t, repo.Update(context.Background(), board))
		require.NoError(t, repo.Delete(context.Background(), 999))
	})
}
