package model

type PostDetail struct {
	Post            *Post
	Tags            []Tag
	Attachments     []Attachment
	Comments        []*CommentDetail
	CommentsHasMore bool
	Reactions       []Reaction
}
