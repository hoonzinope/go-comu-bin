package porttest

import (
	"sync"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func RunReactionRepositoryContractTests(t *testing.T, newRepository func() port.ReactionRepository) {
	t.Helper()

	t.Run("set creates and get by user target returns same reaction", func(t *testing.T) {
		repo := newRepository()

		reaction, created, changed, err := repo.SetUserTargetReaction(7, 10, entity.ReactionTargetPost, entity.ReactionTypeLike)
		require.NoError(t, err)
		require.NotNil(t, reaction)
		assert.True(t, created)
		assert.True(t, changed)

		selected, err := repo.GetUserTargetReaction(7, 10, entity.ReactionTargetPost)
		require.NoError(t, err)
		require.NotNil(t, selected)
		assert.Equal(t, reaction.ID, selected.ID)
		assert.Equal(t, entity.ReactionTypeLike, selected.Type)
	})

	t.Run("set same user target updates instead of duplicating", func(t *testing.T) {
		repo := newRepository()

		first, created, changed, err := repo.SetUserTargetReaction(7, 10, entity.ReactionTargetPost, entity.ReactionTypeLike)
		require.NoError(t, err)
		require.NotNil(t, first)
		assert.True(t, created)
		assert.True(t, changed)

		second, created, changed, err := repo.SetUserTargetReaction(7, 10, entity.ReactionTargetPost, entity.ReactionTypeDislike)
		require.NoError(t, err)
		require.NotNil(t, second)
		assert.False(t, created)
		assert.True(t, changed)
		assert.Equal(t, first.ID, second.ID)
		assert.Equal(t, entity.ReactionTypeDislike, second.Type)

		reactions, err := repo.GetByTarget(10, entity.ReactionTargetPost)
		require.NoError(t, err)
		require.Len(t, reactions, 1)
		assert.Equal(t, entity.ReactionTypeDislike, reactions[0].Type)
	})

	t.Run("set same type is no-op", func(t *testing.T) {
		repo := newRepository()

		first, _, _, err := repo.SetUserTargetReaction(7, 10, entity.ReactionTargetPost, entity.ReactionTypeLike)
		require.NoError(t, err)

		second, created, changed, err := repo.SetUserTargetReaction(7, 10, entity.ReactionTargetPost, entity.ReactionTypeLike)
		require.NoError(t, err)
		require.NotNil(t, second)
		assert.False(t, created)
		assert.False(t, changed)
		assert.Equal(t, first.ID, second.ID)

		reactions, err := repo.GetByTarget(10, entity.ReactionTargetPost)
		require.NoError(t, err)
		require.Len(t, reactions, 1)
	})

	t.Run("delete removes only matching user target reaction", func(t *testing.T) {
		repo := newRepository()

		_, _, _, err := repo.SetUserTargetReaction(7, 10, entity.ReactionTargetPost, entity.ReactionTypeLike)
		require.NoError(t, err)
		_, _, _, err = repo.SetUserTargetReaction(8, 10, entity.ReactionTargetPost, entity.ReactionTypeDislike)
		require.NoError(t, err)

		deleted, err := repo.DeleteUserTargetReaction(7, 10, entity.ReactionTargetPost)
		require.NoError(t, err)
		assert.True(t, deleted)

		selected, err := repo.GetUserTargetReaction(7, 10, entity.ReactionTargetPost)
		require.NoError(t, err)
		assert.Nil(t, selected)

		reactions, err := repo.GetByTarget(10, entity.ReactionTargetPost)
		require.NoError(t, err)
		require.Len(t, reactions, 1)
		assert.Equal(t, int64(8), reactions[0].UserID)
	})

	t.Run("concurrent set preserves uniqueness", func(t *testing.T) {
		repo := newRepository()

		var wg sync.WaitGroup
		errCh := make(chan error, 16)
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func(iter int) {
				defer wg.Done()
				reactionType := entity.ReactionTypeLike
				if iter%2 == 1 {
					reactionType = entity.ReactionTypeDislike
				}
				_, _, _, err := repo.SetUserTargetReaction(7, 10, entity.ReactionTargetPost, reactionType)
				errCh <- err
			}(i)
		}
		wg.Wait()
		close(errCh)
		for err := range errCh {
			require.NoError(t, err)
		}

		reactions, err := repo.GetByTarget(10, entity.ReactionTargetPost)
		require.NoError(t, err)
		require.Len(t, reactions, 1)

		selected, err := repo.GetUserTargetReaction(7, 10, entity.ReactionTargetPost)
		require.NoError(t, err)
		require.NotNil(t, selected)
		assert.Equal(t, reactions[0].ID, selected.ID)
	})
}
