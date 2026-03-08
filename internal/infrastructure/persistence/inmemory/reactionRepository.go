package inmemory

import (
	"strconv"
	"sync"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.ReactionRepository = (*ReactionRepository)(nil)

type ReactionRepository struct {
	mu         sync.RWMutex
	reactionDB struct {
		ID   int64
		Data map[int64]*entity.Reaction
	}
	userTargetIndex map[string]int64
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
		userTargetIndex: make(map[string]int64),
	}
}

func (r *ReactionRepository) SetUserTargetReaction(userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (*entity.Reaction, bool, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	indexKey := userTargetKey(userID, targetID, targetType)
	if reactionID, exists := r.userTargetIndex[indexKey]; exists {
		reaction := r.reactionDB.Data[reactionID]
		if reaction.Type == reactionType {
			return reaction, false, false, nil
		}
		reaction.Update(reactionType)
		return reaction, false, true, nil
	}

	reaction := entity.NewReaction(targetType, targetID, reactionType, userID)
	r.reactionDB.ID++
	reaction.ID = r.reactionDB.ID
	r.reactionDB.Data[reaction.ID] = reaction
	r.userTargetIndex[indexKey] = reaction.ID
	return reaction, true, true, nil
}

func (r *ReactionRepository) DeleteUserTargetReaction(userID, targetID int64, targetType entity.ReactionTargetType) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	indexKey := userTargetKey(userID, targetID, targetType)
	reactionID, exists := r.userTargetIndex[indexKey]
	if !exists {
		return false, nil
	}

	delete(r.userTargetIndex, indexKey)
	delete(r.reactionDB.Data, reactionID)
	return true, nil
}

func (r *ReactionRepository) DeleteByTarget(targetID int64, targetType entity.ReactionTargetType) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	deletedCount := 0
	for reactionID, reaction := range r.reactionDB.Data {
		if reaction.TargetID != targetID || reaction.TargetType != targetType {
			continue
		}
		delete(r.userTargetIndex, userTargetKey(reaction.UserID, reaction.TargetID, reaction.TargetType))
		delete(r.reactionDB.Data, reactionID)
		deletedCount++
	}
	return deletedCount, nil
}

func (r *ReactionRepository) GetUserTargetReaction(userID, targetID int64, targetType entity.ReactionTargetType) (*entity.Reaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reactionID, exists := r.userTargetIndex[userTargetKey(userID, targetID, targetType)]
	if !exists {
		return nil, nil
	}
	return r.reactionDB.Data[reactionID], nil
}

func (r *ReactionRepository) GetByTarget(targetID int64, targetType entity.ReactionTargetType) ([]*entity.Reaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var reactions []*entity.Reaction
	for _, reaction := range r.reactionDB.Data {
		if reaction.TargetID == targetID && reaction.TargetType == targetType {
			reactions = append(reactions, reaction)
		}
	}
	return reactions, nil
}

func userTargetKey(userID, targetID int64, targetType entity.ReactionTargetType) string {
	return string(targetType) + ":" + strconv.FormatInt(targetID, 10) + ":" + strconv.FormatInt(userID, 10)
}
