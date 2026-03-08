package model

type PostDetail struct {
	Post            *Post
	Attachments     []Attachment
	Comments        []*CommentDetail
	CommentsHasMore bool
	Reactions       []Reaction
}
