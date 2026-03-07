package model

type BoardList struct {
	Boards     []Board
	Limit      int
	LastID     int64
	HasMore    bool
	NextLastID *int64
}
