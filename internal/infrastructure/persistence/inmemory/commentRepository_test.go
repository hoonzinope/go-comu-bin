package inmemory

import (
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/porttest"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommentRepositoryContract(t *testing.T) {
	porttest.RunCommentRepositoryContractTests(t, func() port.CommentRepository {
		return NewCommentRepository()
	})
}

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

func TestCommentRepository_Delete_SoftDeletesAndExcludesFromList(t *testing.T) {
	repo := NewCommentRepository()
	id, err := repo.Save(testComment("hello", 1, 1))
	require.NoError(t, err)

	require.NoError(t, repo.Delete(id))

	selected, err := repo.SelectCommentByID(id)
	require.NoError(t, err)
	assert.Nil(t, selected)

	comments, err := repo.SelectComments(1, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, comments)
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

func TestCommentRepository_SelectReturnsClone(t *testing.T) {
	repo := NewCommentRepository()
	id, err := repo.Save(testComment("hello", 1, 1))
	require.NoError(t, err)

	selected, err := repo.SelectCommentByID(id)
	require.NoError(t, err)
	require.NotNil(t, selected)

	selected.Update("mutated outside repository")

	again, err := repo.SelectCommentByID(id)
	require.NoError(t, err)
	require.NotNil(t, again)
	assert.Equal(t, "hello", again.Content)
}

func TestCommentRepository_SelectVisibleComments_AppliesTombstoneFilteringAndLimit(t *testing.T) {
	repo := NewCommentRepository()
	parentID, err := repo.Save(testComment("parent", 1, 1))
	require.NoError(t, err)
	require.NoError(t, repo.Delete(parentID))

	_, err = repo.Save(entity.NewComment("reply", 2, 1, &parentID))
	require.NoError(t, err)

	visible, err := repo.SelectVisibleComments(1, 1, 0)
	require.NoError(t, err)
	require.Len(t, visible, 1)
	assert.Equal(t, entity.CommentStatusActive, visible[0].Status)
}
