package dto

import (
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type PostDetail struct {
	Post      *entity.Post
	Comments  []*CommentDetail
	Reactions []*entity.Reaction
}
