package port

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type ReactionUseCase interface {
	SetReaction(ctx context.Context, userID int64, targetUUID string, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (bool, error)
	DeleteReaction(ctx context.Context, userID int64, targetUUID string, targetType entity.ReactionTargetType) error
	GetReactionsByTarget(ctx context.Context, targetUUID string, targetType entity.ReactionTargetType) ([]model.Reaction, error)
}
