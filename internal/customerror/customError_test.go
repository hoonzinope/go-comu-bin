package customerror

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
