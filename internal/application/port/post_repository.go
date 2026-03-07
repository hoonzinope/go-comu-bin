package port

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type PostRepository interface {
	Save(*entity.Post) (int64, error)
	SelectPostByID(id int64) (*entity.Post, error)
	SelectPosts(boardID int64, limit int, lastID int64) ([]*entity.Post, error)
	Update(*entity.Post) error
	Delete(id int64) error
}
