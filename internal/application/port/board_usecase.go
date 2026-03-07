package port

import "github.com/hoonzinope/go-comu-bin/internal/domain/dto"

type BoardUseCase interface {
	GetBoards(limit int, lastID int64) (*dto.BoardList, error)
	CreateBoard(userID int64, name, description string) (int64, error)
	UpdateBoard(id, userID int64, name, description string) error
	DeleteBoard(id, userID int64) error
}
