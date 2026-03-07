package dto

type CommentList struct {
	Comments   []Comment
	Limit      int
	LastID     int64
	HasMore    bool
	NextLastID *int64
}
