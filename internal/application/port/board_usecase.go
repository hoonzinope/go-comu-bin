package port

import "context"

import "github.com/hoonzinope/go-comu-bin/internal/application/model"

type BoardUseCase interface {
	GetBoards(ctx context.Context, limit int, lastID int64) (*model.BoardList, error)
	CreateBoard(ctx context.Context, userID int64, name, description string) (int64, error)
	UpdateBoard(ctx context.Context, id, userID int64, name, description string) error
	DeleteBoard(ctx context.Context, id, userID int64) error
}
