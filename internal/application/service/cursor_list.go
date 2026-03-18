package service

import "context"

type cursorListPage[T any] struct {
	items      []T
	cursor     string
	hasMore    bool
	nextCursor *string
}

func loadCursorListPage[T any](
	ctx context.Context,
	limit int,
	cursor string,
	lastID int64,
	fetch func(context.Context) ([]T, error),
	itemID func(T) int64,
) (*cursorListPage[T], error) {
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
		next := encodeOpaqueCursor(itemID(fetched[len(fetched)-1]))
		nextCursor = &next
	}
	return &cursorListPage[T]{
		items:      fetched,
		cursor:     cursor,
		hasMore:    hasMore,
		nextCursor: nextCursor,
	}, nil
}
