package port

import "context"

import "github.com/hoonzinope/go-comu-bin/internal/application/model"

type BoardUseCase interface {
	GetBoards(ctx context.Context, limit int, cursor string) (*model.BoardList, error)
	GetAllBoards(ctx context.Context, limit int, cursor string) (*model.BoardList, error)
	CreateBoard(ctx context.Context, userID int64, name, description string) (string, error)
	UpdateBoard(ctx context.Context, boardUUID string, userID int64, name, description string) error
	DeleteBoard(ctx context.Context, boardUUID string, userID int64) error
	SetBoardVisibility(ctx context.Context, boardUUID string, userID int64, hidden bool) error
}
