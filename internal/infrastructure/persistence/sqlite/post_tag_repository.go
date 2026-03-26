package sqlite

import (
	"context"
	"errors"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.PostTagRepository = (*PostTagRepository)(nil)

type PostTagRepository struct {
	db sqlExecutor
}

func NewPostTagRepository(db sqlExecutor) *PostTagRepository {
	return &PostTagRepository{db: db}
}

func (r *PostTagRepository) SelectActiveByPostID(ctx context.Context, postID int64) ([]*entity.PostTag, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("sqlite post tag repository is not initialized")
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT post_id, tag_id, created_at, status
FROM post_tags
WHERE post_id = ? AND status = 'active'
ORDER BY tag_id ASC
`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*entity.PostTag, 0)
	for rows.Next() {
		item, scanErr := scanPostTag(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *PostTagRepository) SelectActiveByTagID(ctx context.Context, tagID int64, limit int, lastID int64) ([]*entity.PostTag, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("sqlite post tag repository is not initialized")
	}
	if limit <= 0 {
		return []*entity.PostTag{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT post_id, tag_id, created_at, status
FROM post_tags
WHERE tag_id = ? AND status = 'active' AND (? <= 0 OR post_id < ?)
ORDER BY post_id DESC
LIMIT ?
`, tagID, lastID, lastID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*entity.PostTag, 0, limit)
	for rows.Next() {
		item, scanErr := scanPostTag(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *PostTagRepository) SelectActivePostIDSetByTagID(tagID int64) map[int64]struct{} {
	if r == nil || r.db == nil {
		return map[int64]struct{}{}
	}
	rows, err := r.db.QueryContext(context.Background(), `
SELECT post_id
FROM post_tags
WHERE tag_id = ? AND status = 'active'
`, tagID)
	if err != nil {
		return map[int64]struct{}{}
	}
	defer rows.Close()
	postIDs := make(map[int64]struct{})
	for rows.Next() {
		var postID int64
		if scanErr := rows.Scan(&postID); scanErr != nil {
			return map[int64]struct{}{}
		}
		postIDs[postID] = struct{}{}
	}
	return postIDs
}

func (r *PostTagRepository) UpsertActive(ctx context.Context, postID, tagID int64) error {
	if r == nil || r.db == nil {
		return errors.New("sqlite post tag repository is not initialized")
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO post_tags (post_id, tag_id, created_at, status)
VALUES (?, ?, ?, 'active')
ON CONFLICT(post_id, tag_id) DO UPDATE SET status = 'active'
`, postID, tagID, time.Now().UnixNano())
	return err
}

func (r *PostTagRepository) SoftDelete(ctx context.Context, postID, tagID int64) error {
	if r == nil || r.db == nil {
		return errors.New("sqlite post tag repository is not initialized")
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE post_tags
SET status = 'deleted'
WHERE post_id = ? AND tag_id = ?
`, postID, tagID)
	return err
}

func (r *PostTagRepository) SoftDeleteByPostID(ctx context.Context, postID int64) error {
	if r == nil || r.db == nil {
		return errors.New("sqlite post tag repository is not initialized")
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE post_tags
SET status = 'deleted'
WHERE post_id = ?
`, postID)
	return err
}
