package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sqlite3 "modernc.org/sqlite/lib"
)

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type sqliteCoder interface {
	Code() int
}

func sqliteErrorCode(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	var coded sqliteCoder
	if !errors.As(err, &coded) {
		return 0, false
	}
	return coded.Code(), true
}

func sqliteBusyError(err error) bool {
	code, ok := sqliteErrorCode(err)
	if !ok {
		return false
	}
	switch code & 0xff {
	case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
		return true
	default:
		return false
	}
}

func sqliteConstraintError(err error) bool {
	code, ok := sqliteErrorCode(err)
	if !ok {
		return false
	}
	switch code & 0xff {
	case sqlite3.SQLITE_CONSTRAINT:
		return true
	case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
		return true
	default:
		return false
	}
}

func sqliteForeignKeyError(err error) bool {
	code, ok := sqliteErrorCode(err)
	if !ok {
		return false
	}
	return code == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY
}

func uniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	if code, ok := sqliteErrorCode(err); ok {
		switch code {
		case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") || strings.Contains(msg, "constraint failed")
}

func timePtrToUnixNano(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UnixNano()
}

func unixNanoToTimePtr(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	t := time.Unix(0, value.Int64).UTC()
	return &t
}

func mustParseSQLTimestamp(name string, value sql.NullInt64) time.Time {
	if !value.Valid {
		panic(fmt.Sprintf("missing required sqlite timestamp column: %s", name))
	}
	return time.Unix(0, value.Int64).UTC()
}
