package inmemory

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ application.ReactionRepository = (*ReactionRepository)(nil)

type ReactionRepository struct {
	reactionDB struct {
		ID   int64
		Data map[int64]*entity.Reaction
	}
}

func NewReactionRepository() *ReactionRepository {
	return &ReactionRepository{
		reactionDB: struct {
			ID   int64
			Data map[int64]*entity.Reaction
		}{
			ID:   0,
			Data: make(map[int64]*entity.Reaction),
		},
	}
}

func (r *ReactionRepository) Add(reaction *entity.Reaction) error {
	r.reactionDB.ID++
	reaction.ID = r.reactionDB.ID
	r.reactionDB.Data[reaction.ID] = reaction
	return nil
}

func (r *ReactionRepository) Remove(reaction *entity.Reaction) error {
	delete(r.reactionDB.Data, reaction.ID)
	return nil
}

func (r *ReactionRepository) GetByTarget(targetID int64, targetType string) ([]*entity.Reaction, error) {
	var reactions []*entity.Reaction
	for _, reaction := range r.reactionDB.Data {
		if reaction.TargetID == targetID && reaction.TargetType == targetType {
			reactions = append(reactions, reaction)
		}
	}
	return reactions, nil
}

func (r *ReactionRepository) GetByID(id int64) (*entity.Reaction, error) {
	if reaction, exists := r.reactionDB.Data[id]; exists {
		return reaction, nil
	}
	return nil, nil
}
