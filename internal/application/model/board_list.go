package model

type BoardList struct {
	Boards     []Board
	Limit      int
	Cursor     string
	HasMore    bool
	NextCursor *string
}
