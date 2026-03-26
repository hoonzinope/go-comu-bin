package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func uniqueConstraintError(err error) bool {
	if err == nil {
		return false
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
