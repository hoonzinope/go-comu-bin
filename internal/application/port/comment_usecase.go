package port

import "context"

import "github.com/hoonzinope/go-comu-bin/internal/application/model"

type CommentUseCase interface {
	CreateComment(ctx context.Context, content string, authorID, postID int64, parentID *int64) (int64, error)
	GetCommentsByPost(ctx context.Context, postID int64, limit int, lastID int64) (*model.CommentList, error)
	UpdateComment(ctx context.Context, id, authorID int64, content string) error
	DeleteComment(ctx context.Context, id, authorID int64) error
}
