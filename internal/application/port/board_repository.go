package port

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type BoardRepository interface {
	SelectBoardByID(id int64) (*entity.Board, error)
	SelectBoardList(limit int, lastID int64) ([]*entity.Board, error)
	Save(*entity.Board) (int64, error)
	Update(*entity.Board) error
	Delete(id int64) error
}
