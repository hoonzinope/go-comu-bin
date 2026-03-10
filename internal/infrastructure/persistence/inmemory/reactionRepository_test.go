package inmemory

import (
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/porttest"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReactionRepositoryContract(t *testing.T) {
	porttest.RunReactionRepositoryContractTests(t, func() port.ReactionRepository {
		return NewReactionRepository()
	})
}

func TestReactionRepository_SelectReturnsClone(t *testing.T) {
	repo := NewReactionRepository()
	reaction, _, _, err := repo.SetUserTargetReaction(7, 10, entity.ReactionTargetPost, entity.ReactionTypeLike)
	require.NoError(t, err)
	require.NotNil(t, reaction)

	selected, err := repo.GetUserTargetReaction(7, 10, entity.ReactionTargetPost)
	require.NoError(t, err)
	require.NotNil(t, selected)

	selected.Update(entity.ReactionTypeDislike)

	again, err := repo.GetUserTargetReaction(7, 10, entity.ReactionTargetPost)
	require.NoError(t, err)
	require.NotNil(t, again)
	assert.Equal(t, entity.ReactionTypeLike, again.Type)
}

func TestReactionRepository_GetByTargets_ReturnsClones(t *testing.T) {
	repo := NewReactionRepository()
	_, _, _, err := repo.SetUserTargetReaction(7, 10, entity.ReactionTargetPost, entity.ReactionTypeLike)
	require.NoError(t, err)

	grouped, err := repo.GetByTargets([]int64{10}, entity.ReactionTargetPost)
	require.NoError(t, err)
	require.Len(t, grouped[10], 1)

	grouped[10][0].Update(entity.ReactionTypeDislike)

	again, err := repo.GetByTarget(10, entity.ReactionTargetPost)
	require.NoError(t, err)
	require.Len(t, again, 1)
	assert.Equal(t, entity.ReactionTypeLike, again[0].Type)
}
