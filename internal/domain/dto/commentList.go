package dto

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type CommentList struct {
	Comments []*entity.Comment
	Limit    int
	Offset   int
}
