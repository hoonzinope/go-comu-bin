package inmemory

import (
	"sort"
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.AttachmentRepository = (*AttachmentRepository)(nil)

type AttachmentRepository struct {
	mu           sync.RWMutex
	coordinator  *txCoordinator
	attachmentDB struct {
		ID   int64
		Data map[int64]*entity.Attachment
	}
}

type attachmentRepositoryState struct {
	ID   int64
	Data map[int64]*entity.Attachment
}

func NewAttachmentRepository() *AttachmentRepository {
	return &AttachmentRepository{
		coordinator: newTxCoordinator(),
		attachmentDB: struct {
			ID   int64
			Data map[int64]*entity.Attachment
		}{
			ID:   0,
			Data: make(map[int64]*entity.Attachment),
		},
	}
}

func (r *AttachmentRepository) attachCoordinator(coordinator *txCoordinator) {
	r.coordinator = coordinator
}

func (r *AttachmentRepository) Save(attachment *entity.Attachment) (int64, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.save(attachment)
}

func (r *AttachmentRepository) save(attachment *entity.Attachment) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.attachmentDB.ID++
	saved := cloneAttachment(attachment)
	saved.ID = r.attachmentDB.ID
	r.attachmentDB.Data[saved.ID] = saved
	attachment.ID = saved.ID
	return saved.ID, nil
}

func (r *AttachmentRepository) SelectByID(id int64) (*entity.Attachment, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectByID(id)
}

func (r *AttachmentRepository) selectByID(id int64) (*entity.Attachment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if attachment, exists := r.attachmentDB.Data[id]; exists {
		return cloneAttachment(attachment), nil
	}
	return nil, nil
}

func (r *AttachmentRepository) SelectByPostID(postID int64) ([]*entity.Attachment, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectByPostID(postID)
}

func (r *AttachmentRepository) selectByPostID(postID int64) ([]*entity.Attachment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*entity.Attachment, 0)
	for _, attachment := range r.attachmentDB.Data {
		if attachment.PostID == postID {
			out = append(out, cloneAttachment(attachment))
		}
	}
	return out, nil
}

func (r *AttachmentRepository) SelectCleanupCandidatesBefore(cutoff time.Time, limit int) ([]*entity.Attachment, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectCleanupCandidatesBefore(cutoff, limit)
}

func (r *AttachmentRepository) selectCleanupCandidatesBefore(cutoff time.Time, limit int) ([]*entity.Attachment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*entity.Attachment, 0)
	for _, attachment := range r.attachmentDB.Data {
		if !attachmentEligibleForCleanup(attachment, cutoff) {
			continue
		}
		out = append(out, cloneAttachment(attachment))
	}
	sort.Slice(out, func(i, j int) bool {
		return cleanupEligibleAt(out[i]).Before(cleanupEligibleAt(out[j]))
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func attachmentEligibleForCleanup(attachment *entity.Attachment, cutoff time.Time) bool {
	eligibleAt := cleanupEligibleAt(attachment)
	if eligibleAt.IsZero() {
		return false
	}
	return !eligibleAt.After(cutoff)
}

func cleanupEligibleAt(attachment *entity.Attachment) time.Time {
	if attachment.PendingDeleteAt != nil {
		return *attachment.PendingDeleteAt
	}
	if attachment.OrphanedAt != nil {
		return *attachment.OrphanedAt
	}
	return time.Time{}
}

func (r *AttachmentRepository) Update(attachment *entity.Attachment) error {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.update(attachment)
}

func (r *AttachmentRepository) update(attachment *entity.Attachment) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if attachment == nil {
		return nil
	}
	r.attachmentDB.Data[attachment.ID] = cloneAttachment(attachment)
	return nil
}

func (r *AttachmentRepository) Delete(id int64) error {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.delete(id)
}

func (r *AttachmentRepository) delete(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.attachmentDB.Data, id)
	return nil
}

func (r *AttachmentRepository) snapshot() attachmentRepositoryState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state := attachmentRepositoryState{
		ID:   r.attachmentDB.ID,
		Data: make(map[int64]*entity.Attachment, len(r.attachmentDB.Data)),
	}
	for id, attachment := range r.attachmentDB.Data {
		state.Data[id] = cloneAttachment(attachment)
	}
	return state
}

func (r *AttachmentRepository) restore(state attachmentRepositoryState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.attachmentDB.ID = state.ID
	r.attachmentDB.Data = make(map[int64]*entity.Attachment, len(state.Data))
	for id, attachment := range state.Data {
		r.attachmentDB.Data[id] = cloneAttachment(attachment)
	}
}

func cloneAttachment(attachment *entity.Attachment) *entity.Attachment {
	if attachment == nil {
		return nil
	}
	out := *attachment
	if attachment.OrphanedAt != nil {
		orphanedAt := *attachment.OrphanedAt
		out.OrphanedAt = &orphanedAt
	}
	if attachment.PendingDeleteAt != nil {
		pendingDeleteAt := *attachment.PendingDeleteAt
		out.PendingDeleteAt = &pendingDeleteAt
	}
	return &out
}
