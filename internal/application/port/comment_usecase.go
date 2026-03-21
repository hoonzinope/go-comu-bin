package port

import "context"

import "github.com/hoonzinope/go-comu-bin/internal/application/model"

type CommentUseCase interface {
	CreateComment(ctx context.Context, content string, mentionedUsernames []string, authorID int64, postUUID string, parentUUID *string) (string, error)
	GetCommentsByPost(ctx context.Context, postUUID string, limit int, cursor string) (*model.CommentList, error)
	UpdateComment(ctx context.Context, commentUUID string, authorID int64, content string) error
	DeleteComment(ctx context.Context, commentUUID string, authorID int64) error
}
