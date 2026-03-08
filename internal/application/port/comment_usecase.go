package port

import "github.com/hoonzinope/go-comu-bin/internal/application/model"

type CommentUseCase interface {
	CreateComment(content string, authorID, postID int64, parentID *int64) (int64, error)
	GetCommentsByPost(postID int64, limit int, lastID int64) (*model.CommentList, error)
	UpdateComment(id, authorID int64, content string) error
	DeleteComment(id, authorID int64) error
}
