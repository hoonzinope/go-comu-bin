package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	sqlitepersist "github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/sqlite"
)

const defaultDriverName = "mysql"

type Options struct {
	DSN          string
	DriverName   string
	Migrations   fs.FS
	MaxOpenConns int
}

type DB struct {
	*sql.DB
}

type Tx struct {
	*sql.Tx
}

func Open(ctx context.Context, opts Options) (*DB, error) {
	if strings.TrimSpace(opts.DSN) == "" {
		return nil, errors.New("mysql dsn is required")
	}
	driverName := strings.TrimSpace(opts.DriverName)
	if driverName == "" {
		driverName = defaultDriverName
	}
	db, err := sql.Open(driverName, opts.DSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql db: %w", err)
	}
	maxOpenConns := opts.MaxOpenConns
	if maxOpenConns <= 0 {
		maxOpenConns = 10
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxOpenConns)
	db.SetConnMaxLifetime(0)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql db: %w", err)
	}
	migrations := opts.Migrations
	if migrations == nil {
		var subErr error
		migrations, subErr = fs.Sub(embeddedMigrations, "migrations")
		if subErr != nil {
			return nil, fmt.Errorf("load embedded mysql migrations: %w", subErr)
		}
	}
	if migrations != nil {
		if err := sqlitepersist.ApplyMigrations(ctx, db, migrations); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	return &DB{DB: db}, nil
}

func (db *DB) WrapTx(tx *sql.Tx) sqlitepersist.Executor {
	return &Tx{Tx: tx}
}

func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	query, args = rewriteQuery(query, args...)
	return db.DB.ExecContext(ctx, query, args...)
}

func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	query, args = rewriteQuery(query, args...)
	return db.DB.QueryContext(ctx, query, args...)
}

func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	query, args = rewriteQuery(query, args...)
	return db.DB.QueryRowContext(ctx, query, args...)
}

func (tx *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	query, args = rewriteQuery(query, args...)
	return tx.Tx.ExecContext(ctx, query, args...)
}

func (tx *Tx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	query, args = rewriteQuery(query, args...)
	return tx.Tx.QueryContext(ctx, query, args...)
}

func (tx *Tx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	query, args = rewriteQuery(query, args...)
	return tx.Tx.QueryRowContext(ctx, query, args...)
}
