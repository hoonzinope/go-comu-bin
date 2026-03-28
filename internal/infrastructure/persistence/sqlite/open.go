package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const defaultDriverName = "sqlite"

var defaultPragmas = []string{
	"PRAGMA foreign_keys = ON",
	"PRAGMA journal_mode = WAL",
	"PRAGMA busy_timeout = 5000",
}

type Options struct {
	Path         string
	DriverName   string
	Migrations   fs.FS
	Pragmas      []string
	MaxOpenConns int
}

func Open(ctx context.Context, opts Options) (*sql.DB, error) {
	if strings.TrimSpace(opts.Path) == "" {
		return nil, errors.New("sqlite path is required")
	}
	driverName := strings.TrimSpace(opts.DriverName)
	if driverName == "" {
		driverName = defaultDriverName
	}
	if err := os.MkdirAll(filepath.Dir(opts.Path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite directory: %w", err)
	}
	db, err := sql.Open(driverName, opts.Path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	maxOpenConns := opts.MaxOpenConns
	if maxOpenConns <= 0 {
		maxOpenConns = 2
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxOpenConns)
	db.SetConnMaxLifetime(0)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}
	pragmas := append([]string{}, defaultPragmas...)
	pragmas = append(pragmas, opts.Pragmas...)
	if err := applyPragmas(ctx, db, pragmas...); err != nil {
		_ = db.Close()
		return nil, err
	}
	migrations := opts.Migrations
	if migrations == nil {
		var subErr error
		migrations, subErr = fs.Sub(embeddedMigrations, "migrations")
		if subErr != nil {
			return nil, fmt.Errorf("load embedded sqlite migrations: %w", subErr)
		}
	}
	if migrations != nil {
		if err := ApplyMigrations(ctx, db, migrations); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	return db, nil
}

func applyPragmas(ctx context.Context, db *sql.DB, pragmas ...string) error {
	for _, pragma := range pragmas {
		pragma = strings.TrimSpace(pragma)
		if pragma == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("apply sqlite pragma %q: %w", pragma, err)
		}
	}
	return nil
}
