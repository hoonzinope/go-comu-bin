package inmemory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_SaveSelectDelete(t *testing.T) {
	repo := NewUserRepository()

	user := testUser("alice", "pw", false)
	id, err := repo.Save(user)
	require.NoError(t, err)
	assert.NotZero(t, id)

	byName, err := repo.SelectUserByUsername("alice")
	require.NoError(t, err)
	require.NotNil(t, byName)
	assert.Equal(t, id, byName.ID)

	byID, err := repo.SelectUserByID(id)
	require.NoError(t, err)
	require.NotNil(t, byID)
	assert.Equal(t, "alice", byID.Name)

	require.NoError(t, repo.Delete(id))
	deleted, err := repo.SelectUserByID(id)
	require.NoError(t, err)
	assert.Nil(t, deleted)
}
