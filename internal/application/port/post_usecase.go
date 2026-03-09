package port

import "github.com/hoonzinope/go-comu-bin/internal/application/model"

type PostUseCase interface {
	CreatePost(title, content string, tags []string, authorID, boardID int64) (int64, error)
	CreateDraftPost(title, content string, tags []string, authorID, boardID int64) (int64, error)
	GetPostsList(boardID int64, limit int, lastID int64) (*model.PostList, error)
	GetPostsByTag(tagName string, limit int, lastID int64) (*model.PostList, error)
	GetPostDetail(postID int64) (*model.PostDetail, error)
	PublishPost(id, authorID int64) error
	UpdatePost(id, authorID int64, title, content string, tags []string) error
	DeletePost(id, authorID int64) error
}
