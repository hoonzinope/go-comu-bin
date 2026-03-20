package common

import "context"

type CursorListPage[T any] struct {
	Items      []T
	Cursor     string
	HasMore    bool
	NextCursor *string
}

func LoadCursorListPage[T any](ctx context.Context, limit int, cursor string, _ int64, fetch func(context.Context) ([]T, error), itemID func(T) int64) (*CursorListPage[T], error) {
	fetched, err := fetch(ctx)
	if err != nil {
		return nil, err
	}
	hasMore := false
	if len(fetched) > limit {
		hasMore = true
		fetched = fetched[:limit]
	}
	var nextCursor *string
	if hasMore && len(fetched) > 0 {
		next := EncodeOpaqueCursor(itemID(fetched[len(fetched)-1]))
		nextCursor = &next
	}
	return &CursorListPage[T]{Items: fetched, Cursor: cursor, HasMore: hasMore, NextCursor: nextCursor}, nil
}
