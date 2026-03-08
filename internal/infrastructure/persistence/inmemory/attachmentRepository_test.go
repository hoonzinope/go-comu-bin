package inmemory

import (
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachmentRepository_SaveListDelete(t *testing.T) {
	repo := NewAttachmentRepository()
	id, err := repo.Save(entity.NewAttachment(1, "a.png", "image/png", 10, "a.png"))
	require.NoError(t, err)

	items, err := repo.SelectByPostID(1)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, id, items[0].ID)

	require.NoError(t, repo.Delete(id))

	items, err = repo.SelectByPostID(1)
	require.NoError(t, err)
	assert.Empty(t, items)
}
