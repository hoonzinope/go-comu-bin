package model

type CommentList struct {
	Comments   []Comment
	Limit      int
	Cursor     string
	HasMore    bool
	NextCursor *string
}
