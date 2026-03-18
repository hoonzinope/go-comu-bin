package port

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
)

type ReactionUseCase interface {
	SetReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType, reactionType model.ReactionType) (bool, error)
	DeleteReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType) error
	GetReactionsByTarget(ctx context.Context, targetUUID string, targetType model.ReactionTargetType) ([]model.Reaction, error)
}
