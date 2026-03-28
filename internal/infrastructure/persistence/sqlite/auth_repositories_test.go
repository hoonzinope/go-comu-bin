package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/porttest"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_Contract(t *testing.T) {
	t.Parallel()

	porttest.RunUserRepositoryContractTests(t, func() port.UserRepository {
		return NewUserRepository(openTestSQLiteDB(t))
	})
}

func TestEmailVerificationTokenRepository_SaveInvalidateAndCleanup(t *testing.T) {
	t.Parallel()

	db := openTestSQLiteDB(t)
	repo := NewEmailVerificationTokenRepository(db)
	now := time.Now()
	first := entity.NewEmailVerificationToken(1, "hash-1", now.Add(time.Hour))
	second := entity.NewEmailVerificationToken(1, "hash-2", now.Add(time.Hour))
	first.CreatedAt = now.Add(-time.Minute)
	second.CreatedAt = now
	require.NoError(t, repo.Save(context.Background(), first))
	require.NoError(t, repo.Save(context.Background(), second))

	loaded, err := repo.SelectByTokenHash(context.Background(), "hash-1")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, int64(1), loaded.UserID)

	latest, err := repo.SelectLatestByUser(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, "hash-2", latest.TokenHash)

	require.NoError(t, repo.InvalidateByUser(context.Background(), 1))
	loaded, err = repo.SelectByTokenHash(context.Background(), "hash-1")
	require.NoError(t, err)
	assert.Nil(t, loaded)

	deleted, err := repo.DeleteExpiredOrConsumedBefore(context.Background(), now.Add(time.Minute), 10)
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)
}

func TestPasswordResetTokenRepository_SaveInvalidateAndCleanup(t *testing.T) {
	t.Parallel()

	db := openTestSQLiteDB(t)
	repo := NewPasswordResetTokenRepository(db)
	now := time.Now()
	first := entity.NewPasswordResetToken(1, "hash-1", now.Add(time.Hour))
	second := entity.NewPasswordResetToken(1, "hash-2", now.Add(time.Hour))
	first.CreatedAt = now.Add(-time.Minute)
	second.CreatedAt = now
	require.NoError(t, repo.Save(context.Background(), first))
	require.NoError(t, repo.Save(context.Background(), second))

	loaded, err := repo.SelectByTokenHash(context.Background(), "hash-1")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, int64(1), loaded.UserID)

	latest, err := repo.SelectLatestByUser(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, "hash-2", latest.TokenHash)

	require.NoError(t, repo.InvalidateByUser(context.Background(), 1))
	loaded, err = repo.SelectByTokenHash(context.Background(), "hash-1")
	require.NoError(t, err)
	assert.Nil(t, loaded)

	deleted, err := repo.DeleteExpiredOrConsumedBefore(context.Background(), now.Add(time.Minute), 10)
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)
}

func TestUnitOfWork_CommitsAndRollsBackAuthChanges(t *testing.T) {
	t.Parallel()

	db := openTestSQLiteDB(t)
	userRepo := NewUserRepository(db)
	emailRepo := NewEmailVerificationTokenRepository(db)
	resetRepo := NewPasswordResetTokenRepository(db)
	uow := NewUnitOfWork(db, nil, nil, nil, nil, nil, nil, nil, nil, nil, emailRepo, resetRepo, nil)

	committedUser := entity.NewUserWithEmail("alice", "alice@example.com", "pw")
	require.NoError(t, uow.WithinTransaction(context.Background(), func(tx port.TxScope) error {
		_, err := tx.UserRepository().Save(context.Background(), committedUser)
		return err
	}))
	loaded, err := userRepo.SelectUserByUsername(context.Background(), "alice")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	rolledBackUser := entity.NewUserWithEmail("bob", "bob@example.com", "pw")
	testErr := errors.New("rollback")
	err = uow.WithinTransaction(context.Background(), func(tx port.TxScope) error {
		_, innerErr := tx.UserRepository().Save(context.Background(), rolledBackUser)
		if innerErr != nil {
			return innerErr
		}
		return testErr
	})
	require.ErrorIs(t, err, testErr)
	loaded, err = userRepo.SelectUserByUsername(context.Background(), "bob")
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

func openTestSQLiteDB(t *testing.T) *sql.DB {
	return openTestSQLiteDBWithMaxOpenConns(t, 1)
}

func openTestSQLiteDBWithMaxOpenConns(t *testing.T, maxOpenConns int) *sql.DB {
	t.Helper()

	tempDir := t.TempDir()
	db, err := Open(context.Background(), Options{
		Path:         tempDir + "/auth.db",
		MaxOpenConns: maxOpenConns,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})
	return db
}
