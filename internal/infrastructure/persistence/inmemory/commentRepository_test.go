package inmemory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommentRepository_FilterByPostAndPagination(t *testing.T) {
	repo := NewCommentRepository()
	_, _ = repo.Save(testComment("c1", 1, 1))
	_, _ = repo.Save(testComment("c2", 2, 1))
	_, _ = repo.Save(testComment("c3", 3, 2))

	comments, err := repo.SelectComments(1, 10, 0)
	require.NoError(t, err)
	assert.Len(t, comments, 2)
	assert.Equal(t, int64(2), comments[0].ID)
	assert.Equal(t, int64(1), comments[1].ID)
}

func TestCommentRepository_SaveSelectUpdateDelete(t *testing.T) {
	repo := NewCommentRepository()
	id, err := repo.Save(testComment("hello", 1, 1))
	require.NoError(t, err)

	selected, err := repo.SelectCommentByID(id)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, "hello", selected.Content)

	selected.Update("updated")
	require.NoError(t, repo.Update(selected))

	updated, err := repo.SelectCommentByID(id)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "updated", updated.Content)

	require.NoError(t, repo.Delete(id))
	deleted, err := repo.SelectCommentByID(id)
	require.NoError(t, err)
	assert.Nil(t, deleted)
}

func TestCommentRepository_PaginationCursorAtEnd_ReturnsEmpty(t *testing.T) {
	repo := NewCommentRepository()
	_, _ = repo.Save(testComment("c1", 1, 1))
	_, _ = repo.Save(testComment("c2", 2, 1))

	comments, err := repo.SelectComments(1, 10, 1)
	require.NoError(t, err)
	assert.Empty(t, comments)
}

func TestCommentRepository_PaginationWithCursor_ReturnsNextChunk(t *testing.T) {
	repo := NewCommentRepository()
	_, _ = repo.Save(testComment("c1", 1, 1))
	_, _ = repo.Save(testComment("c2", 2, 1))
	_, _ = repo.Save(testComment("c3", 3, 1))

	comments, err := repo.SelectComments(1, 10, 3)
	require.NoError(t, err)
	require.Len(t, comments, 2)
	assert.Equal(t, int64(2), comments[0].ID)
	assert.Equal(t, int64(1), comments[1].ID)
}

func TestCommentRepository_UpdateDelete_NonExistingID_NoError(t *testing.T) {
	repo := NewCommentRepository()
	c := testComment("x", 1, 1)
	c.ID = 999

	require.NoError(t, repo.Update(c))
	require.NoError(t, repo.Delete(999))
}
