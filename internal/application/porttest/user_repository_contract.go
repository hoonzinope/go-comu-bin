package porttest

import (
	"sync"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func RunUserRepositoryContractTests(t *testing.T, newRepository func() port.UserRepository) {
	t.Helper()

	t.Run("save and select by id and username", func(t *testing.T) {
		repo := newRepository()

		user := entity.NewUser("alice", "pw")
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
		assert.NotEmpty(t, byID.UUID)

		byUUID, err := repo.SelectUserByUUID(byID.UUID)
		require.NoError(t, err)
		require.NotNil(t, byUUID)
		assert.Equal(t, id, byUUID.ID)
	})

	t.Run("username is unique", func(t *testing.T) {
		repo := newRepository()

		_, err := repo.Save(entity.NewUser("alice", "pw"))
		require.NoError(t, err)

		_, err = repo.Save(entity.NewUser("alice", "pw2"))
		require.Error(t, err)
		assert.ErrorIs(t, err, customError.ErrUserAlreadyExists)
	})

	t.Run("uuid is unique", func(t *testing.T) {
		repo := newRepository()

		user1 := entity.NewUser("alice", "pw")
		user1.UUID = "fixed-uuid"
		_, err := repo.Save(user1)
		require.NoError(t, err)

		user2 := entity.NewUser("bob", "pw")
		user2.UUID = "fixed-uuid"
		_, err = repo.Save(user2)
		require.Error(t, err)
		assert.ErrorIs(t, err, customError.ErrUserAlreadyExists)
	})

	t.Run("concurrent save preserves username uniqueness", func(t *testing.T) {
		repo := newRepository()

		var wg sync.WaitGroup
		errCh := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := repo.Save(entity.NewUser("alice", "pw"))
				errCh <- err
			}()
		}
		wg.Wait()
		close(errCh)

		successes := 0
		failures := 0
		for err := range errCh {
			if err == nil {
				successes++
				continue
			}
			assert.ErrorIs(t, err, customError.ErrUserAlreadyExists)
			failures++
		}

		assert.Equal(t, 1, successes)
		assert.Equal(t, 7, failures)
	})

	t.Run("delete removes only matching user", func(t *testing.T) {
		repo := newRepository()

		aliceID, err := repo.Save(entity.NewUser("alice", "pw"))
		require.NoError(t, err)
		bobID, err := repo.Save(entity.NewUser("bob", "pw"))
		require.NoError(t, err)

		require.NoError(t, repo.Delete(aliceID))

		alice, err := repo.SelectUserByID(aliceID)
		require.NoError(t, err)
		assert.Nil(t, alice)

		bob, err := repo.SelectUserByID(bobID)
		require.NoError(t, err)
		require.NotNil(t, bob)
		assert.Equal(t, "bob", bob.Name)
	})

	t.Run("update persists user soft delete state", func(t *testing.T) {
		repo := newRepository()

		user := entity.NewUser("alice", "pw")
		id, err := repo.Save(user)
		require.NoError(t, err)
		user.ID = id
		user.SoftDelete()

		require.NoError(t, repo.Update(user))

		byID, err := repo.SelectUserByID(id)
		require.NoError(t, err)
		assert.Nil(t, byID)

		byName, err := repo.SelectUserByUsername("alice")
		require.NoError(t, err)
		assert.Nil(t, byName)

		byUUID, err := repo.SelectUserByUUID(user.UUID)
		require.NoError(t, err)
		assert.Nil(t, byUUID)

		includingDeleted, err := repo.SelectUserByIDIncludingDeleted(id)
		require.NoError(t, err)
		require.NotNil(t, includingDeleted)
		assert.Equal(t, user.UUID, includingDeleted.UUID)
	})

	t.Run("select users by ids including deleted returns unique requested users", func(t *testing.T) {
		repo := newRepository()

		aliceID, err := repo.Save(entity.NewUser("alice", "pw"))
		require.NoError(t, err)
		bob := entity.NewUser("bob", "pw")
		bobID, err := repo.Save(bob)
		require.NoError(t, err)
		bob.ID = bobID
		bob.SoftDelete()
		require.NoError(t, repo.Update(bob))

		users, err := repo.SelectUsersByIDsIncludingDeleted([]int64{bobID, aliceID, bobID, 999})
		require.NoError(t, err)
		require.Len(t, users, 2)
		assert.Equal(t, "alice", users[aliceID].Name)
		assert.Equal(t, bob.UUID, users[bobID].UUID)
	})
}
