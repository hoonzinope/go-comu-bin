package port

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type CommentRepository interface {
	Save(*entity.Comment) (int64, error)
	SelectCommentByID(id int64) (*entity.Comment, error)
	SelectComments(postID int64, limit int, lastID int64) ([]*entity.Comment, error)
	SelectCommentsIncludingDeleted(postID int64) ([]*entity.Comment, error)
	SelectVisibleComments(postID int64, limit int, lastID int64) ([]*entity.Comment, error)
	Update(*entity.Comment) error
	Delete(id int64) error
}
