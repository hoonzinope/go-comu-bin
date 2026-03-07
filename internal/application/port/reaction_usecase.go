package port

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type ReactionUseCase interface {
	AddReaction(userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) error
	RemoveReaction(userID, id int64) error
	GetReactionsByTarget(targetID int64, targetType entity.ReactionTargetType) ([]model.Reaction, error)
}
