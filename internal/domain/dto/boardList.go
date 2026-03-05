package dto

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type BoardList struct {
	Boards     []*entity.Board
	Limit      int
	LastID     int64
	HasMore    bool
	NextLastID *int64
}
