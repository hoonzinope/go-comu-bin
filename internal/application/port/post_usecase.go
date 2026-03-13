package port

import "context"

import "github.com/hoonzinope/go-comu-bin/internal/application/model"

type PostUseCase interface {
	CreatePost(ctx context.Context, title, content string, tags []string, authorID, boardID int64) (int64, error)
	CreateDraftPost(ctx context.Context, title, content string, tags []string, authorID, boardID int64) (int64, error)
	GetPostsList(ctx context.Context, boardID int64, limit int, lastID int64) (*model.PostList, error)
	GetPostsByTag(ctx context.Context, tagName string, limit int, lastID int64) (*model.PostList, error)
	GetPostDetail(ctx context.Context, postID int64) (*model.PostDetail, error)
	PublishPost(ctx context.Context, id, authorID int64) error
	UpdatePost(ctx context.Context, id, authorID int64, title, content string, tags []string) error
	DeletePost(ctx context.Context, id, authorID int64) error
}
