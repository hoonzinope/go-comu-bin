package port

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type PostSearchCursor struct {
	Score  float64
	PostID int64
}

type PostSearchResult struct {
	Post  *entity.Post
	Score float64
}

type PostSearchRepository interface {
	SearchPublishedPosts(ctx context.Context, query string, limit int, cursor *PostSearchCursor) ([]PostSearchResult, error)
}
