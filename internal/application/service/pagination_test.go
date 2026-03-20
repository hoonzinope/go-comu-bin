package service

import (
	"errors"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
	"math"
	"testing"

	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequirePositiveLimit(t *testing.T) {
	require.NoError(t, svccommon.RequirePositiveLimit(1))
	require.NoError(t, svccommon.RequirePositiveLimit(svccommon.MaxPageLimit))

	err := svccommon.RequirePositiveLimit(0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))

	err = svccommon.RequirePositiveLimit(svccommon.MaxPageLimit + 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))
}

func TestCursorFetchLimit(t *testing.T) {
	fetch, err := svccommon.CursorFetchLimit(10)
	require.NoError(t, err)
	assert.Equal(t, 11, fetch)

	_, err = svccommon.CursorFetchLimit(svccommon.MaxPageLimit + 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))

	_, err = svccommon.CursorFetchLimit(math.MaxInt)
	require.Error(t, err)
	assert.True(t, errors.Is(err, customerror.ErrInvalidInput))
}
