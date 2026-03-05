package inmemory

import (
	"sort"

	"github.com/hoonzinope/go-comu-bin/internal/application"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ application.BoardRepository = (*BoardRepository)(nil)

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

func (r *BoardRepository) SelectBoardList(limit int, lastID int64) ([]*entity.Board, error) {
	if limit <= 0 {
		return []*entity.Board{}, nil
	}

	var boards []*entity.Board
	for _, board := range r.boardDB.Data {
		if lastID > 0 && board.ID >= lastID {
			continue
		}
		boards = append(boards, board)
	}
	sort.Slice(boards, func(i, j int) bool {
		return boards[i].ID > boards[j].ID
	})

	if len(boards) > limit {
		boards = boards[:limit]
	}
	return boards, nil
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
