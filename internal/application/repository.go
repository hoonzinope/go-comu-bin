package application

import "github.com/hoonzinope/go-comu-bin/internal/domain"

type UserRepository interface {
	// User 관련 Repository 메서드 정의
	SaveUser(username, password string) (int64, error)
	SelectUserByUsername(username string) (*domain.User, error)
	SelectUserByID(id int64) (*domain.User, error)
	DeleteUser(username string) error
}

type BoardRepository interface {
	// Board 관련 Repository 메서드 정의
	SelectBoardByID(id int64) (*domain.Board, error)
	SelectBoardList(limit, offset int) ([]*domain.Board, error)
	SaveBoard(name, description string) (int64, error)
	UpdateBoard(id int64, name, description string) error
	DeleteBoard(id int64) error
}

type PostRepository interface {
	// Post 관련 Repository 메서드 정의
	SavePost(title, content string, authorID, boardID int64) (int64, error)
	SelectPostByID(id int64) (*domain.Post, error)
	SelectPostsByBoardID(boardID int64, limit, offset int) ([]*domain.Post, error)
	UpdatePost(id int64, title, content string) error
	DeletePost(id int64) error
}

type CommentRepository interface {
	// Comment 관련 Repository 메서드 정의
	SaveComment(content string, authorID, postID int64) (int64, error)
	SelectCommentByID(id int64) (*domain.Comment, error)
	SelectCommentsByPostID(postID int64, limit, offset int) ([]*domain.Comment, error)
	UpdateComment(id int64, content string) error
	DeleteComment(id int64) error
}

type ReactionRepository interface {
	// Reaction 관련 Repository 메서드 정의
	AddReaction(userID, targetID int64, targetType, reactionType string) error
	RemoveReaction(userID, targetID int64, targetType string) error
	GetReactionsByTarget(targetID int64, targetType string) ([]*domain.Reaction, error)
}

type Repository struct {
	UserRepository     UserRepository
	BoardRepository    BoardRepository
	PostRepository     PostRepository
	CommentRepository  CommentRepository
	ReactionRepository ReactionRepository
}
