package dto

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type CommentDetail struct {
	Comment   *entity.Comment
	Reactions []*entity.Reaction
}
