package dto

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type CommentList struct {
	Comments   []*entity.Comment
	Limit      int
	LastID     int64
	HasMore    bool
	NextLastID *int64
}
