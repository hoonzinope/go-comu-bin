package sqlite

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	sqlite3 "modernc.org/sqlite/lib"
)

type sqliteCodeErr struct {
	code int
	msg  string
}

func (e sqliteCodeErr) Error() string { return e.msg }
func (e sqliteCodeErr) Code() int     { return e.code }

func TestSQLiteErrorClassification(t *testing.T) {
	t.Run("busy", func(t *testing.T) {
		err := sqliteCodeErr{code: sqlite3.SQLITE_BUSY, msg: "busy"}
		assert.True(t, sqliteBusyError(err))
		assert.False(t, sqliteConstraintError(err))
		assert.False(t, sqliteForeignKeyError(err))
	})

	t.Run("locked counts as busy", func(t *testing.T) {
		err := sqliteCodeErr{code: sqlite3.SQLITE_LOCKED, msg: "locked"}
		assert.True(t, sqliteBusyError(err))
	})

	t.Run("constraint", func(t *testing.T) {
		err := sqliteCodeErr{code: sqlite3.SQLITE_CONSTRAINT, msg: "constraint"}
		assert.True(t, sqliteConstraintError(err))
		assert.False(t, sqliteForeignKeyError(err))
	})

	t.Run("foreign key", func(t *testing.T) {
		err := sqliteCodeErr{code: sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY, msg: "fk"}
		assert.True(t, sqliteForeignKeyError(err))
		assert.True(t, sqliteConstraintError(err))
	})

	t.Run("unique constraint fallback", func(t *testing.T) {
		assert.True(t, uniqueConstraintError(errors.New("UNIQUE constraint failed: users.name")))
		assert.True(t, uniqueConstraintError(sqliteCodeErr{code: sqlite3.SQLITE_CONSTRAINT_UNIQUE, msg: "unique"}))
	})
}
