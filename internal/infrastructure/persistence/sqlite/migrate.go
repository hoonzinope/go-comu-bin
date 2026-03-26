package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

const schemaMigrationsTable = "schema_migrations"

func ApplyMigrations(ctx context.Context, db *sql.DB, migrations fs.FS) error {
	if db == nil {
		return errors.New("sqlite db is required")
	}
	if migrations == nil {
		return nil
	}
	if err := ensureMigrationsTable(ctx, db); err != nil {
		return err
	}
	entries, err := fs.ReadDir(migrations, ".")
	if err != nil {
		return fmt.Errorf("read sqlite migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".sql") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		applied, err := migrationApplied(ctx, db, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyMigrationFile(ctx, db, migrations, name); err != nil {
			return err
		}
	}
	return nil
}

func ensureMigrationsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    name TEXT PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`, schemaMigrationsTable))
	if err != nil {
		return fmt.Errorf("create sqlite migrations table: %w", err)
	}
	return nil
}

func migrationApplied(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var found string
	err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT name FROM %s WHERE name = ?", schemaMigrationsTable), name).Scan(&found)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, fmt.Errorf("check sqlite migration %s: %w", name, err)
	}
}

func applyMigrationFile(ctx context.Context, db *sql.DB, migrations fs.FS, name string) error {
	script, err := fs.ReadFile(migrations, path.Clean(name))
	if err != nil {
		return fmt.Errorf("read sqlite migration %s: %w", name, err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite migration %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, string(script)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply sqlite migration %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s(name) VALUES (?)", schemaMigrationsTable), name); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record sqlite migration %s: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite migration %s: %w", name, err)
	}
	return nil
}
