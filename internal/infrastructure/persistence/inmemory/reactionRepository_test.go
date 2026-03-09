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
