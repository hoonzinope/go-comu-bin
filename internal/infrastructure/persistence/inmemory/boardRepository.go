package inmemory

import (
	"context"
	"sort"
	"sync"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.BoardRepository = (*BoardRepository)(nil)

type BoardRepository struct {
	mu          sync.RWMutex
	coordinator *txCoordinator
	boardDB     struct {
		ID   int64
		Data map[int64]*entity.Board
	}
}

type boardRepositoryState struct {
	ID   int64
	Data map[int64]*entity.Board
}

func NewBoardRepository() *BoardRepository {
	return &BoardRepository{
		coordinator: newTxCoordinator(),
		boardDB: struct {
			ID   int64
			Data map[int64]*entity.Board
		}{
			ID:   0,
			Data: make(map[int64]*entity.Board),
		},
	}
}

func (r *BoardRepository) attachCoordinator(coordinator *txCoordinator) {
	r.coordinator = coordinator
}

func (r *BoardRepository) SelectBoardByID(ctx context.Context, id int64) (*entity.Board, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectBoardByID(id)
}

func (r *BoardRepository) SelectBoardByUUID(ctx context.Context, boardUUID string) (*entity.Board, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectBoardByUUID(boardUUID)
}

func (r *BoardRepository) SelectBoardsByIDs(ctx context.Context, ids []int64) (map[int64]*entity.Board, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectBoardsByIDs(ids)
}

func (r *BoardRepository) selectBoardByID(id int64) (*entity.Board, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if board, exists := r.boardDB.Data[id]; exists {
		return cloneBoard(board), nil
	}
	return nil, nil
}

func (r *BoardRepository) selectBoardsByIDs(ids []int64) (map[int64]*entity.Board, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	selected := make(map[int64]*entity.Board, len(ids))
	for _, id := range ids {
		if board, exists := r.boardDB.Data[id]; exists {
			selected[id] = cloneBoard(board)
		}
	}
	return selected, nil
}

func (r *BoardRepository) selectBoardByUUID(boardUUID string) (*entity.Board, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, board := range r.boardDB.Data {
		if board.UUID == boardUUID {
			return cloneBoard(board), nil
		}
	}
	return nil, nil
}

func (r *BoardRepository) SelectBoardList(ctx context.Context, limit int, lastID int64) ([]*entity.Board, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectBoardList(limit, lastID)
}

func (r *BoardRepository) selectBoardList(limit int, lastID int64) ([]*entity.Board, error) {
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
		if board.Hidden {
			continue
		}
		boards = append(boards, cloneBoard(board))
	}
	sort.Slice(boards, func(i, j int) bool {
		return boards[i].ID > boards[j].ID
	})

	if len(boards) > limit {
		boards = boards[:limit]
	}
	return boards, nil
}

func (r *BoardRepository) Save(ctx context.Context, board *entity.Board) (int64, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.save(board)
}

func (r *BoardRepository) save(board *entity.Board) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.boardDB.ID++
	saved := cloneBoard(board)
	saved.ID = r.boardDB.ID
	r.boardDB.Data[saved.ID] = saved
	board.ID = saved.ID
	return saved.ID, nil
}

func (r *BoardRepository) Update(ctx context.Context, board *entity.Board) error {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.update(board)
}

func (r *BoardRepository) update(board *entity.Board) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.boardDB.Data[board.ID]; exists {
		r.boardDB.Data[board.ID] = cloneBoard(board)
		return nil
	}
	return nil
}

func (r *BoardRepository) Delete(ctx context.Context, id int64) error {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.delete(id)
}

func (r *BoardRepository) delete(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.boardDB.Data, id)
	return nil
}

func (r *BoardRepository) snapshot() boardRepositoryState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state := boardRepositoryState{
		ID:   r.boardDB.ID,
		Data: make(map[int64]*entity.Board, len(r.boardDB.Data)),
	}
	for id, board := range r.boardDB.Data {
		state.Data[id] = cloneBoard(board)
	}
	return state
}

func (r *BoardRepository) restore(state boardRepositoryState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.boardDB.ID = state.ID
	r.boardDB.Data = make(map[int64]*entity.Board, len(state.Data))
	for id, board := range state.Data {
		r.boardDB.Data[id] = cloneBoard(board)
	}
}

func cloneBoard(board *entity.Board) *entity.Board {
	if board == nil {
		return nil
	}
	out := *board
	return &out
}
