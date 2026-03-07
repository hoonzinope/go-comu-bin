package model

type PostDetail struct {
	Post      *Post
	Comments  []*CommentDetail
	Reactions []Reaction
}
