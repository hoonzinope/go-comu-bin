package inmemory

import (
	"context"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/porttest"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachmentRepository_Contract(t *testing.T) {
	porttest.RunAttachmentRepositoryContractTests(t, func() port.AttachmentRepository {
		return NewAttachmentRepository()
	})
}

func TestAttachmentRepository_SaveListDelete(t *testing.T) {
	repo := NewAttachmentRepository()
	id, err := repo.Save(context.Background(), entity.NewAttachment(1, "a.png", "image/png", 10, "a.png"))
	require.NoError(t, err)

	items, err := repo.SelectByPostID(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, id, items[0].ID)

	require.NoError(t, repo.Delete(context.Background(), id))

	items, err = repo.SelectByPostID(context.Background(), 1)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestAttachmentRepository_SelectReturnsClone(t *testing.T) {
	repo := NewAttachmentRepository()
	id, err := repo.Save(context.Background(), entity.NewAttachment(1, "a.png", "image/png", 10, "a.png"))
	require.NoError(t, err)

	selected, err := repo.SelectByID(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, selected)

	selected.MarkPendingDelete()

	again, err := repo.SelectByID(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, again)
	assert.False(t, again.IsPendingDelete())
}
