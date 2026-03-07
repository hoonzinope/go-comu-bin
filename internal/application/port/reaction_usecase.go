package port

import "github.com/hoonzinope/go-comu-bin/internal/application/model"

type ReactionUseCase interface {
	AddReaction(userID, targetID int64, targetType, reactionType string) error
	RemoveReaction(userID, id int64) error
	GetReactionsByTarget(targetID int64, targetType string) ([]model.Reaction, error)
}
