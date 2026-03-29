package port

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type BoardRepository interface {
	SelectBoardByID(ctx context.Context, id int64) (*entity.Board, error)
	SelectBoardByUUID(ctx context.Context, boardUUID string) (*entity.Board, error)
	SelectBoardsByIDs(ctx context.Context, ids []int64) (map[int64]*entity.Board, error)
	SelectBoardList(ctx context.Context, limit int, lastID int64) ([]*entity.Board, error)
	SelectBoardListIncludingHidden(ctx context.Context, limit int, lastID int64) ([]*entity.Board, error)
	Save(ctx context.Context, board *entity.Board) (int64, error)
	Update(ctx context.Context, board *entity.Board) error
	Delete(ctx context.Context, id int64) error
}
