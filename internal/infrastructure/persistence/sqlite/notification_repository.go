package sqlite

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.NotificationRepository = (*NotificationRepository)(nil)

type NotificationRepository struct {
	exec sqlExecutor
}

func NewNotificationRepository(exec sqlExecutor) *NotificationRepository {
	return &NotificationRepository{exec: exec}
}

func (r *NotificationRepository) Save(ctx context.Context, notification *entity.Notification) (int64, error) {
	if r == nil || r.exec == nil {
		return 0, errors.New("sqlite notification repository is not initialized")
	}
	existing, err := r.findExisting(ctx, notification)
	if err != nil {
		return 0, err
	}
	if existing != nil {
		notification.ID = existing.ID
		notification.UUID = existing.UUID
		notification.ReadAt = cloneTimePtr(existing.ReadAt)
		notification.CreatedAt = existing.CreatedAt
		return existing.ID, nil
	}
	res, err := r.exec.ExecContext(ctx, `
INSERT INTO notifications (
    uuid, recipient_user_id, actor_user_id, type, post_id, comment_id,
    actor_name_snapshot, post_title_snapshot, comment_preview_snapshot,
    read_at, created_at, dedup_key
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		notification.UUID,
		notification.RecipientUserID,
		notification.ActorUserID,
		notification.Type,
		notification.PostID,
		notification.CommentID,
		notification.ActorNameSnapshot,
		notification.PostTitleSnapshot,
		notification.CommentPreviewSnapshot,
		timePtrToUnixNano(notification.ReadAt),
		notification.CreatedAt.UnixNano(),
		nullableStringOrNil(notification.DedupKey),
	)
	if err != nil {
		return 0, fmt.Errorf("save notification: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id for notification: %w", err)
	}
	notification.ID = id
	return id, nil
}

func (r *NotificationRepository) SelectByID(ctx context.Context, id int64) (*entity.Notification, error) {
	return r.selectNotification(ctx, `
SELECT id, uuid, recipient_user_id, actor_user_id, type, post_id, comment_id, actor_name_snapshot, post_title_snapshot, comment_preview_snapshot, read_at, created_at, dedup_key
FROM notifications
WHERE id = ?
LIMIT 1
`, id)
}

func (r *NotificationRepository) SelectByUUID(ctx context.Context, notificationUUID string) (*entity.Notification, error) {
	return r.selectNotification(ctx, `
SELECT id, uuid, recipient_user_id, actor_user_id, type, post_id, comment_id, actor_name_snapshot, post_title_snapshot, comment_preview_snapshot, read_at, created_at, dedup_key
FROM notifications
WHERE uuid = ?
LIMIT 1
`, strings.TrimSpace(notificationUUID))
}

func (r *NotificationRepository) SelectByRecipientUserID(ctx context.Context, recipientUserID int64, limit int, lastID int64) ([]*entity.Notification, error) {
	if r == nil || r.exec == nil {
		return nil, errors.New("sqlite notification repository is not initialized")
	}
	if limit <= 0 {
		return []*entity.Notification{}, nil
	}
	query := `
SELECT id, uuid, recipient_user_id, actor_user_id, type, post_id, comment_id, actor_name_snapshot, post_title_snapshot, comment_preview_snapshot, read_at, created_at, dedup_key
FROM notifications
WHERE recipient_user_id = ?
`
	args := []any{recipientUserID}
	if lastID > 0 {
		query += "  AND id < ?\n"
		args = append(args, lastID)
	}
	query += `
ORDER BY id DESC
LIMIT ?
`
	args = append(args, limit)
	items, err := r.selectNotifications(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *NotificationRepository) CountUnreadByRecipientUserID(ctx context.Context, recipientUserID int64) (int, error) {
	if r == nil || r.exec == nil {
		return 0, errors.New("sqlite notification repository is not initialized")
	}
	row := r.exec.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM notifications
WHERE recipient_user_id = ? AND read_at IS NULL
`, recipientUserID)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

func (r *NotificationRepository) MarkRead(ctx context.Context, id int64) error {
	if r == nil || r.exec == nil {
		return errors.New("sqlite notification repository is not initialized")
	}
	_, err := r.exec.ExecContext(ctx, `
UPDATE notifications
SET read_at = COALESCE(read_at, ?)
WHERE id = ?
`, time.Now().UTC().UnixNano(), id)
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	return nil
}

func (r *NotificationRepository) MarkAllReadByRecipientUserID(ctx context.Context, recipientUserID int64) (int, error) {
	if r == nil || r.exec == nil {
		return 0, errors.New("sqlite notification repository is not initialized")
	}
	res, err := r.exec.ExecContext(ctx, `
UPDATE notifications
SET read_at = COALESCE(read_at, ?)
WHERE recipient_user_id = ? AND read_at IS NULL
`, time.Now().UTC().UnixNano(), recipientUserID)
	if err != nil {
		return 0, fmt.Errorf("mark notifications read: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected for mark notifications read: %w", err)
	}
	return int(affected), nil
}

func (r *NotificationRepository) selectNotification(ctx context.Context, query string, args ...any) (*entity.Notification, error) {
	items, err := r.selectNotifications(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return items[0], nil
}

func (r *NotificationRepository) selectNotifications(ctx context.Context, query string, args ...any) ([]*entity.Notification, error) {
	if r == nil || r.exec == nil {
		return nil, errors.New("sqlite notification repository is not initialized")
	}
	rows, err := r.exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select notifications: %w", err)
	}
	defer rows.Close()
	items := make([]*entity.Notification, 0)
	for rows.Next() {
		item, scanErr := scanNotification(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("select notifications: %w", scanErr)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("select notifications: %w", err)
	}
	return items, nil
}

func (r *NotificationRepository) findExisting(ctx context.Context, notification *entity.Notification) (*entity.Notification, error) {
	if notification == nil {
		return nil, nil
	}
	if strings.TrimSpace(notification.DedupKey) != "" {
		return r.selectNotification(ctx, `
SELECT id, uuid, recipient_user_id, actor_user_id, type, post_id, comment_id, actor_name_snapshot, post_title_snapshot, comment_preview_snapshot, read_at, created_at, dedup_key
FROM notifications
WHERE dedup_key = ?
LIMIT 1
`, notification.DedupKey)
	}
	return r.selectNotification(ctx, `
SELECT id, uuid, recipient_user_id, actor_user_id, type, post_id, comment_id, actor_name_snapshot, post_title_snapshot, comment_preview_snapshot, read_at, created_at, dedup_key
FROM notifications
WHERE recipient_user_id = ?
  AND actor_user_id = ?
  AND type = ?
  AND post_id = ?
  AND comment_id = ?
  AND actor_name_snapshot = ?
  AND post_title_snapshot = ?
  AND comment_preview_snapshot = ?
  AND created_at = ?
LIMIT 1
`, notification.RecipientUserID, notification.ActorUserID, notification.Type, notification.PostID, notification.CommentID, notification.ActorNameSnapshot, notification.PostTitleSnapshot, notification.CommentPreviewSnapshot, notification.CreatedAt.UnixNano())
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func nullableStringOrNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
