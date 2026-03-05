package application

import (
	"github.com/hoonzinope/go-comu-bin/internal/domain/dto"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type UserUseCase interface {
	// User 관련 UseCase 메서드 정의
	SignUp(username, password string) (string, error)
	Quit(username, password string) error
	DeleteMe(userID int64, password string) error
	Login(username, password string) (int64, error)
	Logout(username string) error
}

type BoardUseCase interface {
	// Board 관련 UseCase 메서드 정의
	GetBoards(limit int, lastID int64) (*dto.BoardList, error)
	// only admin can create, update, delete board
	CreateBoard(userID int64, name, description string) (int64, error)
	UpdateBoard(id, userID int64, name, description string) error
	DeleteBoard(id, userID int64) error
}

type PostUseCase interface {
	// Post 관련 UseCase 메서드 정의
	CreatePost(title, content string, authorID, boardID int64) (int64, error)
	GetPostsList(boardID int64, limit int, lastID int64) (*dto.PostList, error)
	GetPostDetail(postID int64) (*dto.PostDetail, error)
	// only author can update, delete post
	UpdatePost(id, authorID int64, title, content string) error
	DeletePost(id, authorID int64) error
}

type CommentUseCase interface {
	// Comment 관련 UseCase 메서드 정의
	CreateComment(content string, authorID, postID int64) (int64, error)
	GetCommentsByPost(postID int64, limit int, lastID int64) (*dto.CommentList, error)
	UpdateComment(id, authorID int64, content string) error
	DeleteComment(id, authorID int64) error
}

type ReactionUseCase interface {
	// Reaction 관련 UseCase 메서드 정의
	AddReaction(userID, targetID int64, targetType, reactionType string) error
	RemoveReaction(userID, id int64) error
	GetReactionsByTarget(targetID int64, targetType string) ([]*entity.Reaction, error)
}

type UseCase struct {
	UserUseCase     UserUseCase
	BoardUseCase    BoardUseCase
	PostUseCase     PostUseCase
	CommentUseCase  CommentUseCase
	ReactionUseCase ReactionUseCase
}
