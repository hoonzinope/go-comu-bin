package port

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type ReactionUseCase interface {
	SetReaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (bool, error)
	DeleteReaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType) error
	GetReactionsByTarget(ctx context.Context, targetID int64, targetType entity.ReactionTargetType) ([]model.Reaction, error)
}
