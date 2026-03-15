package model

type PostList struct {
	Posts      []Post
	Limit      int
	Cursor     string
	HasMore    bool
	NextCursor *string
}
