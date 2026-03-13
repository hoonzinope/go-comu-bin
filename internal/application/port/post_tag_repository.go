package port

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type PostTagRepository interface {
	SelectActiveByPostID(ctx context.Context, postID int64) ([]*entity.PostTag, error)
	SelectActiveByTagID(ctx context.Context, tagID int64, limit int, lastID int64) ([]*entity.PostTag, error)
	UpsertActive(ctx context.Context, postID, tagID int64) error
	SoftDelete(ctx context.Context, postID, tagID int64) error
	SoftDeleteByPostID(ctx context.Context, postID int64) error
}
