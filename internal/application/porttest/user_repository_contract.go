package porttest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func RunUserRepositoryContractTests(t *testing.T, newRepository func() port.UserRepository) {
	t.Helper()

	t.Run("save and select by id and username", func(t *testing.T) {
		repo := newRepository()

		user := entity.NewUserWithEmail("alice", "alice@example.com", "pw")
		id, err := repo.Save(context.Background(), user)
		require.NoError(t, err)
		assert.NotZero(t, id)

		byName, err := repo.SelectUserByUsername(context.Background(), "alice")
		require.NoError(t, err)
		require.NotNil(t, byName)
		assert.Equal(t, id, byName.ID)

		byEmail, err := repo.SelectUserByEmail(context.Background(), "alice@example.com")
		require.NoError(t, err)
		require.NotNil(t, byEmail)
		assert.Equal(t, id, byEmail.ID)

		byID, err := repo.SelectUserByID(context.Background(), id)
		require.NoError(t, err)
		require.NotNil(t, byID)
		assert.Equal(t, "alice", byID.Name)
		assert.NotEmpty(t, byID.UUID)

		byUUID, err := repo.SelectUserByUUID(context.Background(), byID.UUID)
		require.NoError(t, err)
		require.NotNil(t, byUUID)
		assert.Equal(t, id, byUUID.ID)
	})

	t.Run("select by username including deleted returns soft deleted user", func(t *testing.T) {
		repo := newRepository()

		user := entity.NewUser("alice", "pw")
		id, err := repo.Save(context.Background(), user)
		require.NoError(t, err)
		user.ID = id
		now := time.Now()
		user.Status = entity.UserStatusDeleted
		user.DeletedAt = &now
		user.UpdatedAt = now
		require.NoError(t, repo.Update(context.Background(), user))

		byName, err := repo.SelectUserByUsername(context.Background(), "alice")
		require.NoError(t, err)
		assert.Nil(t, byName)

		includingDeleted, err := repo.SelectUserByUsernameIncludingDeleted(context.Background(), "alice")
		require.NoError(t, err)
		require.NotNil(t, includingDeleted)
		assert.Equal(t, id, includingDeleted.ID)
		assert.True(t, includingDeleted.IsDeleted())
	})

	t.Run("username is unique", func(t *testing.T) {
		repo := newRepository()

		_, err := repo.Save(context.Background(), entity.NewUser("alice", "pw"))
		require.NoError(t, err)

		_, err = repo.Save(context.Background(), entity.NewUser("alice", "pw2"))
		require.Error(t, err)
		assert.ErrorIs(t, err, customerror.ErrUserAlreadyExists)
	})

	t.Run("email is unique", func(t *testing.T) {
		repo := newRepository()

		_, err := repo.Save(context.Background(), entity.NewUserWithEmail("alice", "alice@example.com", "pw"))
		require.NoError(t, err)

		_, err = repo.Save(context.Background(), entity.NewUserWithEmail("bob", "alice@example.com", "pw2"))
		require.Error(t, err)
		assert.ErrorIs(t, err, customerror.ErrUserAlreadyExists)
	})

	t.Run("uuid is unique", func(t *testing.T) {
		repo := newRepository()

		user1 := entity.NewUser("alice", "pw")
		user1.UUID = "fixed-uuid"
		_, err := repo.Save(context.Background(), user1)
		require.NoError(t, err)

		user2 := entity.NewUser("bob", "pw")
		user2.UUID = "fixed-uuid"
		_, err = repo.Save(context.Background(), user2)
		require.Error(t, err)
		assert.ErrorIs(t, err, customerror.ErrUserAlreadyExists)
	})

	t.Run("concurrent save preserves username uniqueness", func(t *testing.T) {
		repo := newRepository()

		var wg sync.WaitGroup
		errCh := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := repo.Save(context.Background(), entity.NewUser("alice", "pw"))
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
			assert.ErrorIs(t, err, customerror.ErrUserAlreadyExists)
			failures++
		}

		assert.Equal(t, 1, successes)
		assert.Equal(t, 7, failures)
	})

	t.Run("delete removes only matching user", func(t *testing.T) {
		repo := newRepository()

		aliceID, err := repo.Save(context.Background(), entity.NewUser("alice", "pw"))
		require.NoError(t, err)
		bobID, err := repo.Save(context.Background(), entity.NewUser("bob", "pw"))
		require.NoError(t, err)

		require.NoError(t, repo.Delete(context.Background(), aliceID))

		alice, err := repo.SelectUserByID(context.Background(), aliceID)
		require.NoError(t, err)
		assert.Nil(t, alice)

		bob, err := repo.SelectUserByID(context.Background(), bobID)
		require.NoError(t, err)
		require.NotNil(t, bob)
		assert.Equal(t, "bob", bob.Name)
	})

	t.Run("update persists user soft delete state", func(t *testing.T) {
		repo := newRepository()

		user := entity.NewUser("alice", "pw")
		id, err := repo.Save(context.Background(), user)
		require.NoError(t, err)
		user.ID = id
		user.SoftDelete()

		require.NoError(t, repo.Update(context.Background(), user))

		byID, err := repo.SelectUserByID(context.Background(), id)
		require.NoError(t, err)
		assert.Nil(t, byID)

		byName, err := repo.SelectUserByUsername(context.Background(), "alice")
		require.NoError(t, err)
		assert.Nil(t, byName)

		byUUID, err := repo.SelectUserByUUID(context.Background(), user.UUID)
		require.NoError(t, err)
		assert.Nil(t, byUUID)

		includingDeleted, err := repo.SelectUserByIDIncludingDeleted(context.Background(), id)
		require.NoError(t, err)
		require.NotNil(t, includingDeleted)
		assert.Equal(t, user.UUID, includingDeleted.UUID)
	})

	t.Run("select users by ids including deleted returns unique requested users", func(t *testing.T) {
		repo := newRepository()

		aliceID, err := repo.Save(context.Background(), entity.NewUser("alice", "pw"))
		require.NoError(t, err)
		bob := entity.NewUser("bob", "pw")
		bobID, err := repo.Save(context.Background(), bob)
		require.NoError(t, err)
		bob.ID = bobID
		bob.SoftDelete()
		require.NoError(t, repo.Update(context.Background(), bob))

		users, err := repo.SelectUsersByIDsIncludingDeleted(context.Background(), []int64{bobID, aliceID, bobID, 999})
		require.NoError(t, err)
		require.Len(t, users, 2)
		assert.Equal(t, "alice", users[aliceID].Name)
		assert.Equal(t, bob.UUID, users[bobID].UUID)
	})

	t.Run("select guest cleanup candidates includes stale pending expired and unused active guests", func(t *testing.T) {
		repo := newRepository()
		now := time.Now()

		pendingGuest := entity.NewGuest("guest-pending", "guest-pending@example.invalid", "pw")
		pendingTime := now.Add(-2 * time.Hour)
		pendingGuest.GuestIssuedAt = &pendingTime
		pendingID, err := repo.Save(context.Background(), pendingGuest)
		require.NoError(t, err)

		expiredGuest := entity.NewGuest("guest-expired", "guest-expired@example.invalid", "pw")
		expiredGuest.MarkGuestExpired()
		expiredTime := now.Add(-3 * time.Hour)
		expiredGuest.GuestExpiredAt = &expiredTime
		expiredID, err := repo.Save(context.Background(), expiredGuest)
		require.NoError(t, err)

		activeGuest := entity.NewGuest("guest-active", "guest-active@example.invalid", "pw")
		activeGuest.MarkGuestActive()
		activeTime := now.Add(-4 * time.Hour)
		activeGuest.GuestActivatedAt = &activeTime
		activeID, err := repo.Save(context.Background(), activeGuest)
		require.NoError(t, err)

		recentGuest := entity.NewGuest("guest-recent", "guest-recent@example.invalid", "pw")
		recentGuest.MarkGuestActive()
		recentID, err := repo.Save(context.Background(), recentGuest)
		require.NoError(t, err)

		items, err := repo.SelectGuestCleanupCandidates(context.Background(), now, time.Hour, 90*time.Minute, 10)
		require.NoError(t, err)
		require.Len(t, items, 3)
		assert.Equal(t, []int64{activeID, expiredID, pendingID}, []int64{items[0].ID, items[1].ID, items[2].ID})
		assert.NotContains(t, []int64{items[0].ID, items[1].ID, items[2].ID}, recentID)
	})
}
