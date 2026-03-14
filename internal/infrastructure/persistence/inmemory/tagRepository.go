package inmemory

import (
	"context"
	"sync"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.TagRepository = (*TagRepository)(nil)

type TagRepository struct {
	mu             sync.RWMutex
	coordinator    *txCoordinator
	onSelectByName func(context.Context, string)
	tagDB          struct {
		ID     int64
		Data   map[int64]*entity.Tag
		NameTo map[string]int64
	}
}

type tagRepositoryState struct {
	ID     int64
	Data   map[int64]*entity.Tag
	NameTo map[string]int64
}

func NewTagRepository() *TagRepository {
	return &TagRepository{
		coordinator: newTxCoordinator(),
		tagDB: struct {
			ID     int64
			Data   map[int64]*entity.Tag
			NameTo map[string]int64
		}{
			Data:   make(map[int64]*entity.Tag),
			NameTo: make(map[string]int64),
		},
	}
}

func (r *TagRepository) attachCoordinator(coordinator *txCoordinator) {
	r.coordinator = coordinator
}

func (r *TagRepository) Save(ctx context.Context, tag *entity.Tag) (int64, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.save(tag)
}

func (r *TagRepository) save(tag *entity.Tag) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if id, exists := r.tagDB.NameTo[tag.Name]; exists {
		return id, nil
	}
	r.tagDB.ID++
	tag.ID = r.tagDB.ID
	r.tagDB.Data[tag.ID] = cloneTag(tag)
	r.tagDB.NameTo[tag.Name] = tag.ID
	return tag.ID, nil
}

func (r *TagRepository) SelectByName(ctx context.Context, name string) (*entity.Tag, error) {
	if r.onSelectByName != nil {
		r.onSelectByName(ctx, name)
	}
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectByName(name)
}

func (r *TagRepository) selectByName(name string) (*entity.Tag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, exists := r.tagDB.NameTo[name]
	if !exists {
		return nil, nil
	}
	tag, ok := r.tagDB.Data[id]
	if !ok {
		return nil, nil
	}
	return cloneTag(tag), nil
}

func (r *TagRepository) SelectByIDs(ctx context.Context, ids []int64) ([]*entity.Tag, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectByIDs(ids)
}

func (r *TagRepository) selectByIDs(ids []int64) ([]*entity.Tag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*entity.Tag, 0, len(ids))
	for _, id := range ids {
		tag, exists := r.tagDB.Data[id]
		if !exists {
			continue
		}
		out = append(out, cloneTag(tag))
	}
	return out, nil
}

func (r *TagRepository) snapshot() tagRepositoryState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state := tagRepositoryState{
		ID:     r.tagDB.ID,
		Data:   make(map[int64]*entity.Tag, len(r.tagDB.Data)),
		NameTo: make(map[string]int64, len(r.tagDB.NameTo)),
	}
	for id, tag := range r.tagDB.Data {
		state.Data[id] = cloneTag(tag)
	}
	for name, id := range r.tagDB.NameTo {
		state.NameTo[name] = id
	}
	return state
}

func (r *TagRepository) restore(state tagRepositoryState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tagDB.ID = state.ID
	r.tagDB.Data = make(map[int64]*entity.Tag, len(state.Data))
	r.tagDB.NameTo = make(map[string]int64, len(state.NameTo))
	for id, tag := range state.Data {
		r.tagDB.Data[id] = cloneTag(tag)
	}
	for name, id := range state.NameTo {
		r.tagDB.NameTo[name] = id
	}
}

func cloneTag(tag *entity.Tag) *entity.Tag {
	if tag == nil {
		return nil
	}
	out := *tag
	return &out
}
