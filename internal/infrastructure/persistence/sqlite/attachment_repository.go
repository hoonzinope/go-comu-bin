package sqlite

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.AttachmentRepository = (*AttachmentRepository)(nil)

type AttachmentRepository struct {
	exec sqlExecutor
}

func NewAttachmentRepository(exec sqlExecutor) *AttachmentRepository {
	return &AttachmentRepository{exec: exec}
}

func (r *AttachmentRepository) Save(ctx context.Context, attachment *entity.Attachment) (int64, error) {
	if r == nil || r.exec == nil {
		return 0, errors.New("sqlite attachment repository is not initialized")
	}
	res, err := r.exec.ExecContext(ctx, `
INSERT INTO attachments (
    uuid, post_id, file_name, content_type, size_bytes, storage_key, created_at, orphaned_at, pending_delete_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		attachment.UUID,
		attachment.PostID,
		attachment.FileName,
		attachment.ContentType,
		attachment.SizeBytes,
		attachment.StorageKey,
		attachment.CreatedAt.UnixNano(),
		timePtrToUnixNano(attachment.OrphanedAt),
		timePtrToUnixNano(attachment.PendingDeleteAt),
	)
	if err != nil {
		return 0, fmt.Errorf("save attachment: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id for attachment: %w", err)
	}
	attachment.ID = id
	return id, nil
}

func (r *AttachmentRepository) SelectByID(ctx context.Context, id int64) (*entity.Attachment, error) {
	return r.selectAttachment(ctx, `
SELECT id, uuid, post_id, file_name, content_type, size_bytes, storage_key, created_at, orphaned_at, pending_delete_at
FROM attachments
WHERE id = ?
LIMIT 1
`, id)
}

func (r *AttachmentRepository) SelectByUUID(ctx context.Context, attachmentUUID string) (*entity.Attachment, error) {
	return r.selectAttachment(ctx, `
SELECT id, uuid, post_id, file_name, content_type, size_bytes, storage_key, created_at, orphaned_at, pending_delete_at
FROM attachments
WHERE uuid = ?
LIMIT 1
`, strings.TrimSpace(attachmentUUID))
}

func (r *AttachmentRepository) SelectByPostID(ctx context.Context, postID int64) ([]*entity.Attachment, error) {
	return r.selectAttachments(ctx, `
SELECT id, uuid, post_id, file_name, content_type, size_bytes, storage_key, created_at, orphaned_at, pending_delete_at
FROM attachments
WHERE post_id = ?
ORDER BY id DESC
`, postID)
}

func (r *AttachmentRepository) SelectCleanupCandidatesBefore(ctx context.Context, cutoff time.Time, limit int) ([]*entity.Attachment, error) {
	items, err := r.selectAttachments(ctx, `
SELECT id, uuid, post_id, file_name, content_type, size_bytes, storage_key, created_at, orphaned_at, pending_delete_at
FROM attachments
`)
	if err != nil {
		return nil, err
	}
	candidates := make([]*entity.Attachment, 0, len(items))
	for _, item := range items {
		if !attachmentEligibleForCleanup(item, cutoff) {
			continue
		}
		candidates = append(candidates, item)
	}
	sort.Slice(candidates, func(i, j int) bool {
		left := cleanupEligibleAt(candidates[i])
		right := cleanupEligibleAt(candidates[j])
		if left.Equal(right) {
			return candidates[i].ID < candidates[j].ID
		}
		return left.Before(right)
	})
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

func (r *AttachmentRepository) Update(ctx context.Context, attachment *entity.Attachment) error {
	if r == nil || r.exec == nil {
		return errors.New("sqlite attachment repository is not initialized")
	}
	_, err := r.exec.ExecContext(ctx, `
UPDATE attachments SET
    uuid = ?,
    post_id = ?,
    file_name = ?,
    content_type = ?,
    size_bytes = ?,
    storage_key = ?,
    created_at = ?,
    orphaned_at = ?,
    pending_delete_at = ?
WHERE id = ?
`,
		attachment.UUID,
		attachment.PostID,
		attachment.FileName,
		attachment.ContentType,
		attachment.SizeBytes,
		attachment.StorageKey,
		attachment.CreatedAt.UnixNano(),
		timePtrToUnixNano(attachment.OrphanedAt),
		timePtrToUnixNano(attachment.PendingDeleteAt),
		attachment.ID,
	)
	if err != nil {
		return fmt.Errorf("update attachment: %w", err)
	}
	return nil
}

func (r *AttachmentRepository) Delete(ctx context.Context, id int64) error {
	if r == nil || r.exec == nil {
		return errors.New("sqlite attachment repository is not initialized")
	}
	_, err := r.exec.ExecContext(ctx, `DELETE FROM attachments WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}
	return nil
}

func (r *AttachmentRepository) selectAttachment(ctx context.Context, query string, args ...any) (*entity.Attachment, error) {
	items, err := r.selectAttachments(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return items[0], nil
}

func (r *AttachmentRepository) selectAttachments(ctx context.Context, query string, args ...any) ([]*entity.Attachment, error) {
	if r == nil || r.exec == nil {
		return nil, errors.New("sqlite attachment repository is not initialized")
	}
	rows, err := r.exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select attachments: %w", err)
	}
	defer rows.Close()
	items := make([]*entity.Attachment, 0)
	for rows.Next() {
		item, scanErr := scanAttachment(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("select attachments: %w", scanErr)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("select attachments: %w", err)
	}
	return items, nil
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
