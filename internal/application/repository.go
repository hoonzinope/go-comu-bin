package application

import (
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type UserRepository interface {
	// User 관련 Repository 메서드 정의
	Save(*entity.User) (int64, error)
	SelectUserByUsername(username string) (*entity.User, error)
	SelectUserByID(id int64) (*entity.User, error)
	Delete(id int64) error
}

type BoardRepository interface {
	// Board 관련 Repository 메서드 정의
	SelectBoardByID(id int64) (*entity.Board, error)
	SelectBoardList(limit int, lastID int64) ([]*entity.Board, error)
	Save(*entity.Board) (int64, error)
	Update(*entity.Board) error
	Delete(id int64) error
}

type PostRepository interface {
	// Post 관련 Repository 메서드 정의
	Save(*entity.Post) (int64, error)
	SelectPostByID(id int64) (*entity.Post, error)
	SelectPosts(boardID int64, limit int, lastID int64) ([]*entity.Post, error)
	Update(*entity.Post) error
	Delete(id int64) error
}

type CommentRepository interface {
	// Comment 관련 Repository 메서드 정의
	Save(*entity.Comment) (int64, error)
	SelectCommentByID(id int64) (*entity.Comment, error)
	SelectComments(postID int64, limit int, lastID int64) ([]*entity.Comment, error)
	Update(*entity.Comment) error
	Delete(id int64) error
}

type ReactionRepository interface {
	// Reaction 관련 Repository 메서드 정의
	Add(*entity.Reaction) error
	Remove(*entity.Reaction) error
	GetByTarget(targetID int64, targetType string) ([]*entity.Reaction, error)
	GetByID(id int64) (*entity.Reaction, error)
}
