package inmemory

import (
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReactionRepository_AddGetRemove(t *testing.T) {
	repo := NewReactionRepository()
	reaction := testReaction(entity.ReactionTargetPost, 10, entity.ReactionTypeLike, 1)

	require.NoError(t, repo.Add(reaction))
	assert.NotZero(t, reaction.ID)

	byID, err := repo.GetByID(reaction.ID)
	require.NoError(t, err)
	require.NotNil(t, byID)
	assert.Equal(t, entity.ReactionTypeLike, byID.Type)

	list, err := repo.GetByTarget(10, entity.ReactionTargetPost)
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

func TestReactionRepository_Update(t *testing.T) {
	repo := NewReactionRepository()
	reaction := testReaction(entity.ReactionTargetPost, 10, entity.ReactionTypeLike, 1)

	require.NoError(t, repo.Add(reaction))
	reaction.Update(entity.ReactionTypeDislike)
	require.NoError(t, repo.Update(reaction))

	updated, err := repo.GetByID(reaction.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, entity.ReactionTypeDislike, updated.Type)
}
