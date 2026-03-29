package customerror

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	sqlite3 "modernc.org/sqlite/lib"
)

type sqliteCodeError struct {
	code int
	msg  string
}

func (e sqliteCodeError) Error() string { return e.msg }
func (e sqliteCodeError) Code() int     { return e.code }

func TestMarkAndWrapPreserveErrorKind(t *testing.T) {
	root := errors.New("db down")
	err := WrapRepository("select user", root)

	assert.True(t, errors.Is(err, ErrRepositoryFailure))
	assert.True(t, errors.Is(err, root))
	assert.Contains(t, err.Error(), "select user")
}

func TestPublicMapsKnownErrors(t *testing.T) {
	assert.ErrorIs(t, Public(WrapRepository("lookup", ErrAttachmentNotFound)), ErrAttachmentNotFound)
	assert.ErrorIs(t, Public(WrapToken("issue", ErrInvalidToken)), ErrInvalidToken)
	assert.ErrorIs(t, Public(WrapCache("load", ErrForbidden)), ErrForbidden)
	assert.ErrorIs(t, Public(ErrNotFound), ErrNotFound)
	assert.ErrorIs(t, Public(ErrMethodNotAllowed), ErrMethodNotAllowed)
}

func TestPublicFallsBackToInternalServerError(t *testing.T) {
	assert.ErrorIs(t, Public(errors.New("unexpected")), ErrInternalServerError)
}

func TestWrapRepositoryClassifiesSQLiteErrors(t *testing.T) {
	busyErr := WrapRepository("select user", sqliteCodeError{code: sqlite3.SQLITE_BUSY, msg: "busy"})
	assert.ErrorIs(t, busyErr, ErrRepositoryFailure)
	assert.ErrorIs(t, busyErr, ErrSQLiteBusy)
	assert.ErrorIs(t, busyErr, sqliteCodeError{code: sqlite3.SQLITE_BUSY, msg: "busy"})
	assert.ErrorIs(t, Public(busyErr), ErrInternalServerError)

	constraintErr := WrapRepository("save user", sqliteCodeError{code: sqlite3.SQLITE_CONSTRAINT, msg: "constraint"})
	assert.ErrorIs(t, constraintErr, ErrRepositoryFailure)
	assert.ErrorIs(t, constraintErr, ErrSQLiteConstraint)
	assert.ErrorIs(t, Public(constraintErr), ErrInternalServerError)

	fkErr := WrapRepository("delete board", sqliteCodeError{code: sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY, msg: "foreign key"})
	assert.ErrorIs(t, fkErr, ErrRepositoryFailure)
	assert.ErrorIs(t, fkErr, ErrSQLiteForeignKey)
	assert.ErrorIs(t, Public(fkErr), ErrInternalServerError)
}

func TestWrapMailAndStorageKinds(t *testing.T) {
	mailErr := WrapMailDelivery("send verification mail", errors.New("smtp down"))
	assert.ErrorIs(t, mailErr, ErrMailDeliveryFailure)
	assert.ErrorIs(t, Public(mailErr), ErrInternalServerError)

	storageErr := WrapStorage("save attachment file", errors.New("disk full"))
	assert.ErrorIs(t, storageErr, ErrStorageFailure)
	assert.ErrorIs(t, Public(storageErr), ErrInternalServerError)
}
