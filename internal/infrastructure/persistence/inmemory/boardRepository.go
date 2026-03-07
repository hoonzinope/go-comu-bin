package inmemory

import (
	"sort"
	"sync"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.BoardRepository = (*BoardRepository)(nil)

type BoardRepository struct {
	mu      sync.RWMutex
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
	r.mu.RLock()
	defer r.mu.RUnlock()

	if board, exists := r.boardDB.Data[id]; exists {
		return board, nil
	}
	return nil, nil
}

func (r *BoardRepository) SelectBoardList(limit int, lastID int64) ([]*entity.Board, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

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
	r.mu.Lock()
	defer r.mu.Unlock()

	r.boardDB.ID++
	board.ID = r.boardDB.ID
	r.boardDB.Data[board.ID] = board
	return board.ID, nil
}

func (r *BoardRepository) Update(board *entity.Board) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.boardDB.Data[board.ID]; exists {
		r.boardDB.Data[board.ID] = board
		return nil
	}
	return nil
}

func (r *BoardRepository) Delete(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.boardDB.Data, id)
	return nil
}
