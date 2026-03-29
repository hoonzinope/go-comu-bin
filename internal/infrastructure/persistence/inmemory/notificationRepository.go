package inmemory

import (
	"context"
	"container/heap"
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.NotificationRepository = (*NotificationRepository)(nil)

type NotificationRepository struct {
	mu          sync.RWMutex
	coordinator *txCoordinator
	db          struct {
		ID   int64
		Data map[int64]*entity.Notification
	}
}

type notificationRepositoryState struct {
	ID   int64
	Data map[int64]*entity.Notification
}

func NewNotificationRepository() *NotificationRepository {
	return &NotificationRepository{
		coordinator: newTxCoordinator(),
		db: struct {
			ID   int64
			Data map[int64]*entity.Notification
		}{
			Data: make(map[int64]*entity.Notification),
		},
	}
}

func (r *NotificationRepository) attachCoordinator(coordinator *txCoordinator) {
	r.coordinator = coordinator
}

func (r *NotificationRepository) Save(ctx context.Context, notification *entity.Notification) (int64, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.save(notification)
}

func (r *NotificationRepository) save(notification *entity.Notification) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, existing := range r.db.Data {
		if sameNotificationDedup(existing, notification) {
			notification.ID = id
			notification.UUID = existing.UUID
			notification.ReadAt = cloneTimePointer(existing.ReadAt)
			notification.CreatedAt = existing.CreatedAt
			return id, nil
		}
	}
	r.db.ID++
	saved := cloneNotification(notification)
	saved.ID = r.db.ID
	if saved.UUID == "" {
		saved.UUID = entity.NewNotification(
			saved.RecipientUserID,
			saved.ActorUserID,
			saved.Type,
			saved.PostID,
			saved.CommentID,
			saved.ActorNameSnapshot,
			saved.PostTitleSnapshot,
			saved.CommentPreviewSnapshot,
		).UUID
	}
	r.db.Data[saved.ID] = saved
	notification.ID = saved.ID
	notification.UUID = saved.UUID
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = saved.CreatedAt
	}
	return saved.ID, nil
}

func (r *NotificationRepository) SelectByID(ctx context.Context, id int64) (*entity.Notification, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()

	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneNotification(r.db.Data[id]), nil
}

func (r *NotificationRepository) SelectByUUID(ctx context.Context, notificationUUID string) (*entity.Notification, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()

	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, item := range r.db.Data {
		if item.UUID == notificationUUID {
			return cloneNotification(item), nil
		}
	}
	return nil, nil
}

func (r *NotificationRepository) SelectByRecipientUserID(ctx context.Context, recipientUserID int64, limit int, lastID int64) ([]*entity.Notification, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()

	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 {
		return []*entity.Notification{}, nil
	}
	items := &notificationHeap{}
	for _, item := range r.db.Data {
		if item.RecipientUserID != recipientUserID {
			continue
		}
		if lastID > 0 && item.ID >= lastID {
			continue
		}
		cloned := cloneNotification(item)
		if items.Len() < limit {
			heap.Push(items, cloned)
			continue
		}
		if cloned.ID <= (*items)[0].ID {
			continue
		}
		heap.Pop(items)
		heap.Push(items, cloned)
	}
	out := make([]*entity.Notification, items.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(items).(*entity.Notification)
	}
	return out, nil
}

func (r *NotificationRepository) CountUnreadByRecipientUserID(ctx context.Context, recipientUserID int64) (int, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()

	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, item := range r.db.Data {
		if item.RecipientUserID == recipientUserID && item.ReadAt == nil {
			count++
		}
	}
	return count, nil
}

func (r *NotificationRepository) MarkRead(ctx context.Context, id int64) error {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.markRead(id)
}

func (r *NotificationRepository) MarkAllReadByRecipientUserID(ctx context.Context, recipientUserID int64) (int, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()

	r.mu.Lock()
	defer r.mu.Unlock()
	changed := 0
	for _, item := range r.db.Data {
		if item.RecipientUserID != recipientUserID || item.ReadAt != nil {
			continue
		}
		item.MarkRead()
		changed++
	}
	return changed, nil
}

func (r *NotificationRepository) markRead(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.db.Data[id]
	if !ok {
		return nil
	}
	item.MarkRead()
	return nil
}

func (r *NotificationRepository) snapshot() notificationRepositoryState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state := notificationRepositoryState{
		ID:   r.db.ID,
		Data: make(map[int64]*entity.Notification, len(r.db.Data)),
	}
	for id, item := range r.db.Data {
		state.Data[id] = cloneNotification(item)
	}
	return state
}

func (r *NotificationRepository) restore(state notificationRepositoryState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.db.ID = state.ID
	r.db.Data = make(map[int64]*entity.Notification, len(state.Data))
	for id, item := range state.Data {
		r.db.Data[id] = cloneNotification(item)
	}
}

func cloneNotification(item *entity.Notification) *entity.Notification {
	if item == nil {
		return nil
	}
	out := *item
	out.ReadAt = cloneTimePointer(item.ReadAt)
	return &out
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func sameNotificationDedup(left, right *entity.Notification) bool {
	if left == nil || right == nil {
		return false
	}
	if left.DedupKey != "" || right.DedupKey != "" {
		return left.DedupKey != "" && left.DedupKey == right.DedupKey
	}
	return left.RecipientUserID == right.RecipientUserID &&
		left.ActorUserID == right.ActorUserID &&
		left.Type == right.Type &&
		left.PostID == right.PostID &&
		left.CommentID == right.CommentID &&
		left.ActorNameSnapshot == right.ActorNameSnapshot &&
		left.PostTitleSnapshot == right.PostTitleSnapshot &&
		left.CommentPreviewSnapshot == right.CommentPreviewSnapshot &&
		left.CreatedAt.Equal(right.CreatedAt)
}

type notificationHeap []*entity.Notification

func (h notificationHeap) Len() int { return len(h) }

func (h notificationHeap) Less(i, j int) bool { return h[i].ID < h[j].ID }

func (h notificationHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *notificationHeap) Push(x any) { *h = append(*h, x.(*entity.Notification)) }

func (h *notificationHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}
