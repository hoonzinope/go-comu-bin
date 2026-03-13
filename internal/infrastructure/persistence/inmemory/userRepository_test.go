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

func TestUserRepositoryContract(t *testing.T) {
	porttest.RunUserRepositoryContractTests(t, func() port.UserRepository {
		return NewUserRepository()
	})
}

func TestUserRepository_SelectReturnsClone(t *testing.T) {
	repo := NewUserRepository()
	id, err := repo.Save(context.Background(), entity.NewUser("alice", "pw"))
	require.NoError(t, err)

	selected, err := repo.SelectUserByID(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, selected)

	selected.Name = "mutated"

	again, err := repo.SelectUserByID(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, again)
	assert.Equal(t, "alice", again.Name)
}

func TestUserRepository_SelectUsersByIDsIncludingDeleted_ReturnsClones(t *testing.T) {
	repo := NewUserRepository()
	id, err := repo.Save(context.Background(), entity.NewUser("alice", "pw"))
	require.NoError(t, err)

	users, err := repo.SelectUsersByIDsIncludingDeleted(context.Background(), []int64{id})
	require.NoError(t, err)
	require.Contains(t, users, id)

	users[id].Name = "mutated"

	again, err := repo.SelectUsersByIDsIncludingDeleted(context.Background(), []int64{id})
	require.NoError(t, err)
	require.Contains(t, again, id)
	assert.Equal(t, "alice", again[id].Name)
}
