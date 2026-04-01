package model

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type CommentDetail struct {
	Comment        *Comment
	Reactions      []Reaction
	MyReactionType *entity.ReactionType
}
