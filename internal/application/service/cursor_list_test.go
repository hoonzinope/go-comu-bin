package service

import (
	"context"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cursorListItem struct {
	id int64
}

func TestLoadCursorListPage(t *testing.T) {
	page, err := svccommon.LoadCursorListPage(context.Background(), 2, "cursor-1", 1, func(context.Context) ([]cursorListItem, error) {
		return []cursorListItem{{id: 3}, {id: 2}, {id: 1}}, nil
	}, func(item cursorListItem) int64 {
		return item.id
	})
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	assert.True(t, page.HasMore)
	require.NotNil(t, page.NextCursor)
	assert.Equal(t, svccommon.EncodeOpaqueCursor(2), *page.NextCursor)
	assert.Equal(t, "cursor-1", page.Cursor)
}

func TestLoadCursorListPage_PropagatesFetcherError(t *testing.T) {
	_, err := svccommon.LoadCursorListPage(context.Background(), 2, "", 0, func(context.Context) ([]cursorListItem, error) {
		return nil, assert.AnError
	}, func(item cursorListItem) int64 {
		return item.id
	})
	require.ErrorIs(t, err, assert.AnError)
}
