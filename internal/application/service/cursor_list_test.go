package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cursorListItem struct {
	id int64
}

func TestLoadCursorListPage(t *testing.T) {
	page, err := loadCursorListPage(context.Background(), 2, "cursor-1", 1, func(context.Context) ([]cursorListItem, error) {
		return []cursorListItem{{id: 3}, {id: 2}, {id: 1}}, nil
	}, func(item cursorListItem) int64 {
		return item.id
	})
	require.NoError(t, err)
	require.Len(t, page.items, 2)
	assert.True(t, page.hasMore)
	require.NotNil(t, page.nextCursor)
	assert.Equal(t, encodeOpaqueCursor(2), *page.nextCursor)
	assert.Equal(t, "cursor-1", page.cursor)
}

func TestLoadCursorListPage_PropagatesFetcherError(t *testing.T) {
	_, err := loadCursorListPage(context.Background(), 2, "", 0, func(context.Context) ([]cursorListItem, error) {
		return nil, assert.AnError
	}, func(item cursorListItem) int64 {
		return item.id
	})
	require.ErrorIs(t, err, assert.AnError)
}
