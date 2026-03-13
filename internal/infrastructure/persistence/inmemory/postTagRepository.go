package inmemory

import (
	"context"
	"sort"
	"sync"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.PostTagRepository = (*PostTagRepository)(nil)

type postTagKey struct {
	PostID int64
	TagID  int64
}

type PostTagRepository struct {
	mu          sync.RWMutex
	coordinator *txCoordinator
	data        map[postTagKey]*entity.PostTag
}

type postTagRepositoryState struct {
	Data map[postTagKey]*entity.PostTag
}

func NewPostTagRepository() *PostTagRepository {
	return &PostTagRepository{
		coordinator: newTxCoordinator(),
		data:        make(map[postTagKey]*entity.PostTag),
	}
}

func (r *PostTagRepository) attachCoordinator(coordinator *txCoordinator) {
	r.coordinator = coordinator
}

func (r *PostTagRepository) SelectActiveByPostID(ctx context.Context, postID int64) ([]*entity.PostTag, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectActiveByPostID(postID)
}

func (r *PostTagRepository) selectActiveByPostID(postID int64) ([]*entity.PostTag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var items []*entity.PostTag
	for _, item := range r.data {
		if item.PostID == postID && item.Status == entity.PostTagStatusActive {
			items = append(items, clonePostTag(item))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].TagID == items[j].TagID {
			return items[i].PostID < items[j].PostID
		}
		return items[i].TagID < items[j].TagID
	})
	return items, nil
}

func (r *PostTagRepository) SelectActiveByTagID(ctx context.Context, tagID int64, limit int, lastID int64) ([]*entity.PostTag, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectActiveByTagID(tagID, limit, lastID)
}

func (r *PostTagRepository) selectActiveByTagID(tagID int64, limit int, lastID int64) ([]*entity.PostTag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		return []*entity.PostTag{}, nil
	}
	var items []*entity.PostTag
	for _, item := range r.data {
		if item.TagID != tagID || item.Status != entity.PostTagStatusActive {
			continue
		}
		if lastID > 0 && item.PostID >= lastID {
			continue
		}
		items = append(items, clonePostTag(item))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].PostID > items[j].PostID
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *PostTagRepository) SelectActivePostIDSetByTagID(tagID int64) map[int64]struct{} {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectActivePostIDSetByTagID(tagID)
}

func (r *PostTagRepository) selectActivePostIDSetByTagID(tagID int64) map[int64]struct{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	postIDs := make(map[int64]struct{})
	for _, item := range r.data {
		if item.TagID == tagID && item.Status == entity.PostTagStatusActive {
			postIDs[item.PostID] = struct{}{}
		}
	}
	return postIDs
}

func (r *PostTagRepository) UpsertActive(ctx context.Context, postID, tagID int64) error {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.upsertActive(postID, tagID)
}

func (r *PostTagRepository) upsertActive(postID, tagID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := postTagKey{PostID: postID, TagID: tagID}
	if item, exists := r.data[key]; exists {
		item.Activate()
		r.data[key] = item
		return nil
	}
	r.data[key] = entity.NewPostTag(postID, tagID)
	return nil
}

func (r *PostTagRepository) SoftDelete(ctx context.Context, postID, tagID int64) error {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.softDelete(postID, tagID)
}

func (r *PostTagRepository) softDelete(postID, tagID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := postTagKey{PostID: postID, TagID: tagID}
	item, exists := r.data[key]
	if !exists {
		return nil
	}
	item.SoftDelete()
	r.data[key] = item
	return nil
}

func (r *PostTagRepository) SoftDeleteByPostID(ctx context.Context, postID int64) error {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.softDeleteByPostID(postID)
}

func (r *PostTagRepository) softDeleteByPostID(postID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, item := range r.data {
		if item.PostID != postID {
			continue
		}
		item.SoftDelete()
		r.data[key] = item
	}
	return nil
}

func (r *PostTagRepository) snapshot() postTagRepositoryState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state := postTagRepositoryState{Data: make(map[postTagKey]*entity.PostTag, len(r.data))}
	for key, item := range r.data {
		state.Data[key] = clonePostTag(item)
	}
	return state
}

func (r *PostTagRepository) restore(state postTagRepositoryState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data = make(map[postTagKey]*entity.PostTag, len(state.Data))
	for key, item := range state.Data {
		r.data[key] = clonePostTag(item)
	}
}

func clonePostTag(item *entity.PostTag) *entity.PostTag {
	if item == nil {
		return nil
	}
	out := *item
	return &out
}
