package policy

import (
	"errors"
	"testing"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
)

func TestEnsureBoardVisible(t *testing.T) {
	t.Run("nil board returns not found", func(t *testing.T) {
		err := EnsureBoardVisible(nil, nil)
		assert.True(t, errors.Is(err, customError.ErrBoardNotFound))
	})

	t.Run("visible board is allowed", func(t *testing.T) {
		err := EnsureBoardVisible(&entity.Board{Hidden: false}, nil)
		assert.NoError(t, err)
	})

	t.Run("hidden board is denied for anonymous", func(t *testing.T) {
		err := EnsureBoardVisible(&entity.Board{Hidden: true}, nil)
		assert.True(t, errors.Is(err, customError.ErrBoardNotFound))
	})

	t.Run("hidden board is denied for non-admin", func(t *testing.T) {
		err := EnsureBoardVisible(&entity.Board{Hidden: true}, &entity.User{Role: "user"})
		assert.True(t, errors.Is(err, customError.ErrBoardNotFound))
	})

	t.Run("hidden board is allowed for admin", func(t *testing.T) {
		err := EnsureBoardVisible(&entity.Board{Hidden: true}, &entity.User{Role: "admin"})
		assert.NoError(t, err)
	})
}
