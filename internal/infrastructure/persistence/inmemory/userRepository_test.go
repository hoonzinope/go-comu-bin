package inmemory

import (
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
	id, err := repo.Save(entity.NewUser("alice", "pw"))
	require.NoError(t, err)

	selected, err := repo.SelectUserByID(id)
	require.NoError(t, err)
	require.NotNil(t, selected)

	selected.Name = "mutated"

	again, err := repo.SelectUserByID(id)
	require.NoError(t, err)
	require.NotNil(t, again)
	assert.Equal(t, "alice", again.Name)
}
