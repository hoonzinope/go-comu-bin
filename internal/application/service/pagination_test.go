package service

import (
	"errors"
	"math"
	"testing"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequirePositiveLimit(t *testing.T) {
	require.NoError(t, requirePositiveLimit(1))
	require.NoError(t, requirePositiveLimit(maxPageLimit))

	err := requirePositiveLimit(0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))

	err = requirePositiveLimit(maxPageLimit + 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}

func TestCursorFetchLimit(t *testing.T) {
	fetch, err := cursorFetchLimit(10)
	require.NoError(t, err)
	assert.Equal(t, 11, fetch)

	_, err = cursorFetchLimit(maxPageLimit + 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))

	_, err = cursorFetchLimit(math.MaxInt)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customError.ErrInvalidInput))
}
