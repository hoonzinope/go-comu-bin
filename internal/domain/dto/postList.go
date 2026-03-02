package dto

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type PostList struct {
	Posts  []*entity.Post
	Limit  int
	Offset int
}
