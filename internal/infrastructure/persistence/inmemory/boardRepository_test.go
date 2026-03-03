package inmemory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoardRepository_ListPagination(t *testing.T) {
	repo := NewBoardRepository()
	_, _ = repo.Save(testBoard("b1", "d1"))
	_, _ = repo.Save(testBoard("b2", "d2"))
	_, _ = repo.Save(testBoard("b3", "d3"))

	boards, err := repo.SelectBoardList(2, 1)
	require.NoError(t, err)
	assert.Len(t, boards, 2)
}

func TestBoardRepository_SaveSelectUpdateDelete(t *testing.T) {
	repo := NewBoardRepository()
	id, err := repo.Save(testBoard("free", "desc"))
	require.NoError(t, err)

	selected, err := repo.SelectBoardByID(id)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, "free", selected.Name)

	selected.UpdateBoard("new", "new-desc")
	require.NoError(t, repo.Update(selected))

	updated, err := repo.SelectBoardByID(id)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "new", updated.Name)

	require.NoError(t, repo.Delete(id))
	deleted, err := repo.SelectBoardByID(id)
	require.NoError(t, err)
	assert.Nil(t, deleted)
}

func TestBoardRepository_PaginationOffsetEqualsLen_ReturnsEmpty(t *testing.T) {
	repo := NewBoardRepository()
	_, _ = repo.Save(testBoard("b1", "d1"))
	_, _ = repo.Save(testBoard("b2", "d2"))

	boards, err := repo.SelectBoardList(10, 2)
	require.NoError(t, err)
	assert.Empty(t, boards)
}

func TestBoardRepository_UpdateDelete_NonExistingID_NoError(t *testing.T) {
	repo := NewBoardRepository()
	b := testBoard("x", "y")
	b.ID = 999

	require.NoError(t, repo.Update(b))
	require.NoError(t, repo.Delete(999))
}
