package inmemory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostRepository_FilterByBoardAndPagination(t *testing.T) {
	repo := NewPostRepository()
	_, _ = repo.Save(testPost("p1", "c1", 1, 1))
	_, _ = repo.Save(testPost("p2", "c2", 1, 1))
	_, _ = repo.Save(testPost("p3", "c3", 2, 2))

	posts, err := repo.SelectPosts(1, 10, 0)
	require.NoError(t, err)
	assert.Len(t, posts, 2)
	assert.Equal(t, int64(2), posts[0].ID)
	assert.Equal(t, int64(1), posts[1].ID)
}

func TestPostRepository_SaveSelectUpdateDelete(t *testing.T) {
	repo := NewPostRepository()
	id, err := repo.Save(testPost("title", "content", 1, 1))
	require.NoError(t, err)

	selected, err := repo.SelectPostByID(id)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, "title", selected.Title)

	selected.Update("new", "new-content")
	require.NoError(t, repo.Update(selected))

	updated, err := repo.SelectPostByID(id)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "new", updated.Title)

	require.NoError(t, repo.Delete(id))
	deleted, err := repo.SelectPostByID(id)
	require.NoError(t, err)
	assert.Nil(t, deleted)
}

func TestPostRepository_Delete_SoftDeletesAndExcludesFromList(t *testing.T) {
	repo := NewPostRepository()
	id, err := repo.Save(testPost("title", "content", 1, 1))
	require.NoError(t, err)

	require.NoError(t, repo.Delete(id))

	selected, err := repo.SelectPostByID(id)
	require.NoError(t, err)
	assert.Nil(t, selected)

	posts, err := repo.SelectPosts(1, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, posts)
}

func TestPostRepository_PaginationCursorAtEnd_ReturnsEmpty(t *testing.T) {
	repo := NewPostRepository()
	_, _ = repo.Save(testPost("p1", "c1", 1, 1))
	_, _ = repo.Save(testPost("p2", "c2", 1, 1))

	posts, err := repo.SelectPosts(1, 10, 1)
	require.NoError(t, err)
	assert.Empty(t, posts)
}

func TestPostRepository_PaginationWithCursor_ReturnsNextChunk(t *testing.T) {
	repo := NewPostRepository()
	_, _ = repo.Save(testPost("p1", "c1", 1, 1))
	_, _ = repo.Save(testPost("p2", "c2", 1, 1))
	_, _ = repo.Save(testPost("p3", "c3", 1, 1))

	posts, err := repo.SelectPosts(1, 10, 3)
	require.NoError(t, err)
	require.Len(t, posts, 2)
	assert.Equal(t, int64(2), posts[0].ID)
	assert.Equal(t, int64(1), posts[1].ID)
}

func TestPostRepository_UpdateDelete_NonExistingID_NoError(t *testing.T) {
	repo := NewPostRepository()
	p := testPost("x", "y", 1, 1)
	p.ID = 999

	require.NoError(t, repo.Update(p))
	require.NoError(t, repo.Delete(999))
}
