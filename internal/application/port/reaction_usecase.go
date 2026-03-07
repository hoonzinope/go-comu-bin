package port

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type ReactionUseCase interface {
	SetReaction(userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (bool, error)
	DeleteReaction(userID, targetID int64, targetType entity.ReactionTargetType) error
	GetReactionsByTarget(targetID int64, targetType entity.ReactionTargetType) ([]model.Reaction, error)
}
