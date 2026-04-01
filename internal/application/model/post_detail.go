package model

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type PostDetail struct {
	Post            *Post
	Tags            []Tag
	Attachments     []Attachment
	Comments        []*CommentDetail
	CommentsHasMore bool
	Reactions       []Reaction
	MyReactionType  *entity.ReactionType
}
