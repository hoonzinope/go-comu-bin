package port

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type ReactionRepository interface {
	Add(*entity.Reaction) error
	Update(*entity.Reaction) error
	Remove(*entity.Reaction) error
	GetByTarget(targetID int64, targetType entity.ReactionTargetType) ([]*entity.Reaction, error)
	GetByID(id int64) (*entity.Reaction, error)
}
