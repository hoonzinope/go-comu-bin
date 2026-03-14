package port

import "context"

import "github.com/hoonzinope/go-comu-bin/internal/application/model"

type OutboxAdminUseCase interface {
	GetDeadMessages(ctx context.Context, adminID int64, limit int, lastID string) (*model.OutboxDeadMessageList, error)
	RequeueDeadMessage(ctx context.Context, adminID int64, messageID string) error
	DiscardDeadMessage(ctx context.Context, adminID int64, messageID string) error
}

