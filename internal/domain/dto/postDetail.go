package dto

type PostDetail struct {
	Post      *Post
	Comments  []*CommentDetail
	Reactions []Reaction
}
