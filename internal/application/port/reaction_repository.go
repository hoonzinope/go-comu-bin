package port

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type ReactionRepository interface {
	SetUserTargetReaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (*entity.Reaction, bool, bool, error)
	DeleteUserTargetReaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType) (bool, error)
	DeleteByTarget(ctx context.Context, targetID int64, targetType entity.ReactionTargetType) (int, error)
	GetUserTargetReaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType) (*entity.Reaction, error)
	GetByTarget(ctx context.Context, targetID int64, targetType entity.ReactionTargetType) ([]*entity.Reaction, error)
	GetByTargets(ctx context.Context, targetIDs []int64, targetType entity.ReactionTargetType) (map[int64][]*entity.Reaction, error)
	ExistsByUserID(ctx context.Context, userID int64) (bool, error)
}
