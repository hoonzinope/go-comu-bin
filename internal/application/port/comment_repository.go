package port

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type CommentRepository interface {
	Save(ctx context.Context, comment *entity.Comment) (int64, error)
	SelectCommentByID(ctx context.Context, id int64) (*entity.Comment, error)
	SelectCommentByUUID(ctx context.Context, commentUUID string) (*entity.Comment, error)
	SelectCommentUUIDsByIDsIncludingDeleted(ctx context.Context, ids []int64) (map[int64]string, error)
	SelectComments(ctx context.Context, postID int64, limit int, lastID int64) ([]*entity.Comment, error)
	SelectCommentsIncludingDeleted(ctx context.Context, postID int64) ([]*entity.Comment, error)
	SelectVisibleComments(ctx context.Context, postID int64, limit int, lastID int64) ([]*entity.Comment, error)
	ExistsByAuthorID(ctx context.Context, authorID int64) (bool, error)
	ExistsByAuthorIDIncludingDeleted(ctx context.Context, authorID int64) (bool, error)
	Update(ctx context.Context, comment *entity.Comment) error
	Delete(ctx context.Context, id int64) error
}
