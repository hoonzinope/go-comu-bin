package model

type PostList struct {
	Posts      []Post
	Limit      int
	Cursor     string
	TotalCount int
	HasMore    bool
	NextCursor *string
}
