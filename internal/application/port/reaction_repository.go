package port

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type ReactionRepository interface {
	SetUserTargetReaction(userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (*entity.Reaction, bool, bool, error)
	DeleteUserTargetReaction(userID, targetID int64, targetType entity.ReactionTargetType) (bool, error)
	GetUserTargetReaction(userID, targetID int64, targetType entity.ReactionTargetType) (*entity.Reaction, error)
	GetByTarget(targetID int64, targetType entity.ReactionTargetType) ([]*entity.Reaction, error)
}
