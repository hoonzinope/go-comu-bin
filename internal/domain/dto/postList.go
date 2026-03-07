package dto

type PostList struct {
	Posts      []Post
	Limit      int
	LastID     int64
	HasMore    bool
	NextLastID *int64
}
