package inmemory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReactionRepository_AddGetRemove(t *testing.T) {
	repo := NewReactionRepository()
	reaction := testReaction("post", 10, "like", 1)

	require.NoError(t, repo.Add(reaction))
	assert.NotZero(t, reaction.ID)

	byID, err := repo.GetByID(reaction.ID)
	require.NoError(t, err)
	require.NotNil(t, byID)
	assert.Equal(t, "like", byID.Type)

	list, err := repo.GetByTarget(10, "post")
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, repo.Remove(reaction))
	deleted, err := repo.GetByID(reaction.ID)
	require.NoError(t, err)
	assert.Nil(t, deleted)
}

func TestReactionRepository_GetMissing_ReturnsNil(t *testing.T) {
	repo := NewReactionRepository()

	r, err := repo.GetByID(999)
	require.NoError(t, err)
	assert.Nil(t, r)
}
