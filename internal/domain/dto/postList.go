package dto

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type PostList struct {
	Posts      []*entity.Post
	Limit      int
	LastID     int64
	HasMore    bool
	NextLastID *int64
}
