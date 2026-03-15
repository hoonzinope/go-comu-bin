package port

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type PostRepository interface {
	Save(ctx context.Context, post *entity.Post) (int64, error)
	SelectPostByID(ctx context.Context, id int64) (*entity.Post, error)
	SelectPostByUUID(ctx context.Context, postUUID string) (*entity.Post, error)
	SelectPostByIDIncludingUnpublished(ctx context.Context, id int64) (*entity.Post, error)
	SelectPostByUUIDIncludingUnpublished(ctx context.Context, postUUID string) (*entity.Post, error)
	SelectPosts(ctx context.Context, boardID int64, limit int, lastID int64) ([]*entity.Post, error)
	SelectPublishedPostsByTagName(ctx context.Context, tagName string, limit int, lastID int64) ([]*entity.Post, error)
	ExistsByBoardID(ctx context.Context, boardID int64) (bool, error)
	Update(ctx context.Context, post *entity.Post) error
	Delete(ctx context.Context, id int64) error
}
