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
	attachmentDB struct {
		ID   int64
		Data map[int64]*entity.Attachment
	}
}

func NewAttachmentRepository() *AttachmentRepository {
	return &AttachmentRepository{
		attachmentDB: struct {
			ID   int64
			Data map[int64]*entity.Attachment
		}{
			ID:   0,
			Data: make(map[int64]*entity.Attachment),
		},
	}
}

func (r *AttachmentRepository) Save(attachment *entity.Attachment) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.attachmentDB.ID++
	attachment.ID = r.attachmentDB.ID
	r.attachmentDB.Data[attachment.ID] = attachment
	return attachment.ID, nil
}

func (r *AttachmentRepository) SelectByID(id int64) (*entity.Attachment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if attachment, exists := r.attachmentDB.Data[id]; exists {
		return attachment, nil
	}
	return nil, nil
}

func (r *AttachmentRepository) SelectByPostID(postID int64) ([]*entity.Attachment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*entity.Attachment, 0)
	for _, attachment := range r.attachmentDB.Data {
		if attachment.PostID == postID {
			out = append(out, attachment)
		}
	}
	return out, nil
}

func (r *AttachmentRepository) SelectOrphansBefore(cutoff time.Time, limit int) ([]*entity.Attachment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*entity.Attachment, 0)
	for _, attachment := range r.attachmentDB.Data {
		if attachment.OrphanedAt == nil {
			continue
		}
		if attachment.OrphanedAt.After(cutoff) {
			continue
		}
		out = append(out, attachment)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].OrphanedAt.Before(*out[j].OrphanedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *AttachmentRepository) Update(attachment *entity.Attachment) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if attachment == nil {
		return nil
	}
	r.attachmentDB.Data[attachment.ID] = attachment
	return nil
}

func (r *AttachmentRepository) Delete(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.attachmentDB.Data, id)
	return nil
}
