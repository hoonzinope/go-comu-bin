package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestOpenAppliesMigrationsInOrderAndSkipsAppliedMigrations(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dbPath := tempDir + "/app.db"
	migrations := fstest.MapFS{
		"0001_create_users.sql": {
			Data: []byte(`
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE
);
`),
		},
		"0002_seed_users.sql": {
			Data: []byte(`
INSERT INTO users (username) VALUES ('alice');
`),
		},
	}

	db, err := Open(context.Background(), Options{
		Path:       dbPath,
		Migrations: migrations,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	requireMigrationCount(t, db, 2)
	requireUserCount(t, db, 1)

	reopened, err := Open(context.Background(), Options{
		Path:       dbPath,
		Migrations: migrations,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, reopened.Close())
	})

	requireMigrationCount(t, reopened, 2)
	requireUserCount(t, reopened, 1)
}

func TestOpenRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	_, err := Open(context.Background(), Options{})
	require.Error(t, err)
}

func TestOpenUsesDefaultConnectionPoolSize(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	dbPath := tempDir + "/app.db"

	db, err := Open(context.Background(), Options{Path: dbPath})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	require.Equal(t, 2, db.Stats().MaxOpenConnections)
}

func requireMigrationCount(t *testing.T, db *sql.DB, expected int) {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count))
	require.Equal(t, expected, count)
}

func requireUserCount(t *testing.T, db *sql.DB, expected int) {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count))
	require.Equal(t, expected, count)
}
