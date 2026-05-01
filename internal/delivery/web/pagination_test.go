package web

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPaginationData(t *testing.T) {
	t.Run("first page with more pages", func(t *testing.T) {
		pagination := buildPaginationData(1, true)
		require.NotNil(t, pagination)
		assert.Equal(t, 1, pagination.Page)
		assert.Equal(t, 0, pagination.PrevPage)
		assert.Equal(t, 2, pagination.NextPage)
		assert.Equal(t, []int{1, 2, 3}, pagination.Pages)
	})

	t.Run("last page without more pages", func(t *testing.T) {
		pagination := buildPaginationData(5, false)
		require.NotNil(t, pagination)
		assert.Equal(t, 5, pagination.Page)
		assert.Equal(t, 4, pagination.PrevPage)
		assert.Equal(t, 0, pagination.NextPage)
		assert.Equal(t, []int{3, 4, 5}, pagination.Pages)
	})

	t.Run("uses total pages when available", func(t *testing.T) {
		pagination := buildPaginationData(3, false, 4)
		require.NotNil(t, pagination)
		assert.Equal(t, 3, pagination.Page)
		assert.Equal(t, 2, pagination.PrevPage)
		assert.Equal(t, 4, pagination.NextPage)
		assert.Equal(t, []int{1, 2, 3, 4}, pagination.Pages)
	})

	t.Run("caps total pages at the browser max", func(t *testing.T) {
		pagination := buildPaginationData(1000, false, 1200)
		require.NotNil(t, pagination)
		assert.Equal(t, 1000, pagination.Page)
		assert.Equal(t, 999, pagination.PrevPage)
		assert.Equal(t, 0, pagination.NextPage)
		assert.Len(t, pagination.Pages, 1000)
		assert.Equal(t, 1, pagination.Pages[0])
		assert.Equal(t, 1000, pagination.Pages[len(pagination.Pages)-1])
	})
}

func TestLoadSequentialPage(t *testing.T) {
	t.Run("walks pages sequentially until target page", func(t *testing.T) {
		calls := 0
		result, currentPage, hasMore, err := loadSequentialPage(context.Background(), 3, "", func(ctx context.Context, cursor string) (string, string, bool, error) {
			_ = ctx
			calls++
			switch calls {
			case 1:
				assert.Empty(t, cursor)
				return "page-1", "cursor-1", true, nil
			case 2:
				assert.Equal(t, "cursor-1", cursor)
				return "page-2", "cursor-2", true, nil
			case 3:
				assert.Equal(t, "cursor-2", cursor)
				return "page-3", "cursor-3", false, nil
			default:
				return "", "", false, errors.New("unexpected extra fetch")
			}
		})
		require.NoError(t, err)
		assert.Equal(t, "page-3", result)
		assert.Equal(t, 3, currentPage)
		assert.False(t, hasMore)
		assert.Equal(t, 3, calls)
	})

	t.Run("stops at the last available page when target exceeds available pages", func(t *testing.T) {
		calls := 0
		result, currentPage, hasMore, err := loadSequentialPage(context.Background(), 8, "", func(ctx context.Context, cursor string) (string, string, bool, error) {
			_ = ctx
			_ = cursor
			calls++
			if calls == 1 {
				return "page-1", "cursor-1", true, nil
			}
			if calls == 2 {
				return "page-2", "cursor-2", false, nil
			}
			return "", "", false, errors.New("unexpected extra fetch")
		})
		require.NoError(t, err)
		assert.Equal(t, "page-2", result)
		assert.Equal(t, 2, currentPage)
		assert.False(t, hasMore)
		assert.Equal(t, 2, calls)
	})
}
