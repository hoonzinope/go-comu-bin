package inmemory

import (
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type BoardRepository struct {
	boardDB struct {
		ID   int64
		Data map[int64]*entity.Board
	}
}

func NewBoardRepository() *BoardRepository {
	return &BoardRepository{
		boardDB: struct {
			ID   int64
			Data map[int64]*entity.Board
		}{
			ID:   0,
			Data: make(map[int64]*entity.Board),
		},
	}
}

func (r *BoardRepository) SelectBoardByID(id int64) (*entity.Board, error) {
	if board, exists := r.boardDB.Data[id]; exists {
		return board, nil
	}
	return nil, nil
}

func (r *BoardRepository) SelectBoardList(limit, offset int) ([]*entity.Board, error) {
	var boards []*entity.Board
	for _, board := range r.boardDB.Data {
		boards = append(boards, board)
	}
	if offset > len(boards) {
		return []*entity.Board{}, nil
	}
	end := offset + limit
	if end > len(boards) {
		end = len(boards)
	}
	return boards[offset:end], nil
}

func (r *BoardRepository) Save(board *entity.Board) (int64, error) {
	r.boardDB.ID++
	board.ID = r.boardDB.ID
	r.boardDB.Data[board.ID] = board
	return board.ID, nil
}

func (r *BoardRepository) Update(board *entity.Board) error {
	if _, exists := r.boardDB.Data[board.ID]; exists {
		r.boardDB.Data[board.ID] = board
		return nil
	}
	return nil
}

func (r *BoardRepository) Delete(id int64) error {
	delete(r.boardDB.Data, id)
	return nil
}
