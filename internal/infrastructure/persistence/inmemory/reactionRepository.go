package inmemory

import (
	"context"
	"strconv"
	"sync"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.ReactionRepository = (*ReactionRepository)(nil)

type ReactionRepository struct {
	mu          sync.RWMutex
	coordinator *txCoordinator
	reactionDB  struct {
		ID   int64
		Data map[int64]*entity.Reaction
	}
	userTargetIndex map[string]int64
}

type reactionRepositoryState struct {
	ID              int64
	Data            map[int64]*entity.Reaction
	UserTargetIndex map[string]int64
}

func NewReactionRepository() *ReactionRepository {
	return &ReactionRepository{
		coordinator: newTxCoordinator(),
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

func (r *ReactionRepository) attachCoordinator(coordinator *txCoordinator) {
	r.coordinator = coordinator
}

func (r *ReactionRepository) SetUserTargetReaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (*entity.Reaction, bool, bool, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.setUserTargetReaction(userID, targetID, targetType, reactionType)
}

func (r *ReactionRepository) setUserTargetReaction(userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (*entity.Reaction, bool, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	indexKey := userTargetKey(userID, targetID, targetType)
	if reactionID, exists := r.userTargetIndex[indexKey]; exists {
		stored := r.reactionDB.Data[reactionID]
		if stored.Type == reactionType {
			return cloneReaction(stored), false, false, nil
		}
		updated := cloneReaction(stored)
		updated.Update(reactionType)
		r.reactionDB.Data[reactionID] = updated
		return cloneReaction(updated), false, true, nil
	}

	reaction := entity.NewReaction(targetType, targetID, reactionType, userID)
	r.reactionDB.ID++
	reaction.ID = r.reactionDB.ID
	r.reactionDB.Data[reaction.ID] = cloneReaction(reaction)
	r.userTargetIndex[indexKey] = reaction.ID
	return cloneReaction(reaction), true, true, nil
}

func (r *ReactionRepository) DeleteUserTargetReaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType) (bool, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.deleteUserTargetReaction(userID, targetID, targetType)
}

func (r *ReactionRepository) deleteUserTargetReaction(userID, targetID int64, targetType entity.ReactionTargetType) (bool, error) {
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

func (r *ReactionRepository) DeleteByTarget(ctx context.Context, targetID int64, targetType entity.ReactionTargetType) (int, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.deleteByTarget(targetID, targetType)
}

func (r *ReactionRepository) deleteByTarget(targetID int64, targetType entity.ReactionTargetType) (int, error) {
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

func (r *ReactionRepository) GetUserTargetReaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType) (*entity.Reaction, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.getUserTargetReaction(userID, targetID, targetType)
}

func (r *ReactionRepository) getUserTargetReaction(userID, targetID int64, targetType entity.ReactionTargetType) (*entity.Reaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reactionID, exists := r.userTargetIndex[userTargetKey(userID, targetID, targetType)]
	if !exists {
		return nil, nil
	}
	return cloneReaction(r.reactionDB.Data[reactionID]), nil
}

func (r *ReactionRepository) GetByTarget(ctx context.Context, targetID int64, targetType entity.ReactionTargetType) ([]*entity.Reaction, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.getByTarget(targetID, targetType)
}

func (r *ReactionRepository) getByTarget(targetID int64, targetType entity.ReactionTargetType) ([]*entity.Reaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var reactions []*entity.Reaction
	for _, reaction := range r.reactionDB.Data {
		if reaction.TargetID == targetID && reaction.TargetType == targetType {
			reactions = append(reactions, cloneReaction(reaction))
		}
	}
	return reactions, nil
}

func (r *ReactionRepository) GetByTargets(ctx context.Context, targetIDs []int64, targetType entity.ReactionTargetType) (map[int64][]*entity.Reaction, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.getByTargets(targetIDs, targetType)
}

func (r *ReactionRepository) getByTargets(targetIDs []int64, targetType entity.ReactionTargetType) (map[int64][]*entity.Reaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	targetSet := make(map[int64]struct{}, len(targetIDs))
	for _, targetID := range targetIDs {
		targetSet[targetID] = struct{}{}
	}

	out := make(map[int64][]*entity.Reaction, len(targetSet))
	for _, reaction := range r.reactionDB.Data {
		if reaction.TargetType != targetType {
			continue
		}
		if _, ok := targetSet[reaction.TargetID]; !ok {
			continue
		}
		out[reaction.TargetID] = append(out[reaction.TargetID], cloneReaction(reaction))
	}
	return out, nil
}

func userTargetKey(userID, targetID int64, targetType entity.ReactionTargetType) string {
	return string(targetType) + ":" + strconv.FormatInt(targetID, 10) + ":" + strconv.FormatInt(userID, 10)
}

func (r *ReactionRepository) snapshot() reactionRepositoryState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state := reactionRepositoryState{
		ID:              r.reactionDB.ID,
		Data:            make(map[int64]*entity.Reaction, len(r.reactionDB.Data)),
		UserTargetIndex: make(map[string]int64, len(r.userTargetIndex)),
	}
	for id, reaction := range r.reactionDB.Data {
		state.Data[id] = cloneReaction(reaction)
	}
	for key, id := range r.userTargetIndex {
		state.UserTargetIndex[key] = id
	}
	return state
}

func (r *ReactionRepository) restore(state reactionRepositoryState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.reactionDB.ID = state.ID
	r.reactionDB.Data = make(map[int64]*entity.Reaction, len(state.Data))
	r.userTargetIndex = make(map[string]int64, len(state.UserTargetIndex))
	for id, reaction := range state.Data {
		r.reactionDB.Data[id] = cloneReaction(reaction)
	}
	for key, id := range state.UserTargetIndex {
		r.userTargetIndex[key] = id
	}
}

func cloneReaction(reaction *entity.Reaction) *entity.Reaction {
	if reaction == nil {
		return nil
	}
	out := *reaction
	return &out
}
