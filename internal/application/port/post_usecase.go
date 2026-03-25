package port

import "context"

import "github.com/hoonzinope/go-comu-bin/internal/application/model"

type PostUseCase interface {
	CreatePost(ctx context.Context, title, content string, tags []string, mentionedUsernames []string, authorID int64, boardUUID string) (string, error)
	CreateDraftPost(ctx context.Context, title, content string, tags []string, mentionedUsernames []string, authorID int64, boardUUID string) (string, error)
	GetPostsList(ctx context.Context, boardUUID string, sort string, window string, limit int, cursor string) (*model.PostList, error)
	GetFeed(ctx context.Context, sort string, window string, limit int, cursor string) (*model.PostList, error)
	SearchPosts(ctx context.Context, query string, sort string, window string, limit int, cursor string) (*model.PostList, error)
	GetPostsByTag(ctx context.Context, tagName string, sort string, window string, limit int, cursor string) (*model.PostList, error)
	GetPostDetail(ctx context.Context, postUUID string) (*model.PostDetail, error)
	PublishPost(ctx context.Context, postUUID string, authorID int64) error
	UpdatePost(ctx context.Context, postUUID string, authorID int64, title, content string, tags []string) error
	DeletePost(ctx context.Context, postUUID string, authorID int64) error
}
