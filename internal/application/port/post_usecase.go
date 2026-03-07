package port

import "github.com/hoonzinope/go-comu-bin/internal/domain/dto"

type PostUseCase interface {
	CreatePost(title, content string, authorID, boardID int64) (int64, error)
	GetPostsList(boardID int64, limit int, lastID int64) (*dto.PostList, error)
	GetPostDetail(postID int64) (*dto.PostDetail, error)
	UpdatePost(id, authorID int64, title, content string) error
	DeletePost(id, authorID int64) error
}
