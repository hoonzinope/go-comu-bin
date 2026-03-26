package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.PostRepository = (*PostRepository)(nil)

type PostRepository struct {
	db sqlExecutor
}

func NewPostRepository(db sqlExecutor) *PostRepository {
	return &PostRepository{db: db}
}

func (r *PostRepository) Save(ctx context.Context, post *entity.Post) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("sqlite post repository is not initialized")
	}
	res, err := r.db.ExecContext(ctx, `
INSERT INTO posts (
    uuid, title, content, author_id, board_id, status, created_at, published_at, updated_at, deleted_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, post.UUID, post.Title, post.Content, post.AuthorID, post.BoardID, post.Status, post.CreatedAt.UnixNano(), timePtrToUnixNano(post.PublishedAt), post.UpdatedAt.UnixNano(), timePtrToUnixNano(post.DeletedAt))
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	post.ID = id
	return id, nil
}

func (r *PostRepository) SelectPostByID(ctx context.Context, id int64) (*entity.Post, error) {
	return r.selectPost(ctx, `
SELECT id, uuid, title, content, author_id, board_id, status, created_at, published_at, updated_at, deleted_at
FROM posts
WHERE id = ? AND status = 'published'
`, id)
}

func (r *PostRepository) SelectPostByUUID(ctx context.Context, postUUID string) (*entity.Post, error) {
	return r.selectPost(ctx, `
SELECT id, uuid, title, content, author_id, board_id, status, created_at, published_at, updated_at, deleted_at
FROM posts
WHERE uuid = ? AND status = 'published'
`, postUUID)
}

func (r *PostRepository) SelectPostUUIDsByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	return r.selectPostUUIDsByIDs(ctx, ids, false)
}

func (r *PostRepository) SelectPostUUIDsByIDsIncludingDeleted(ctx context.Context, ids []int64) (map[int64]string, error) {
	return r.selectPostUUIDsByIDs(ctx, ids, true)
}

func (r *PostRepository) SelectPostsByIDsIncludingUnpublished(ctx context.Context, ids []int64) (map[int64]*entity.Post, error) {
	return r.selectPostsByIDs(ctx, ids, false)
}

func (r *PostRepository) SelectPostByIDIncludingUnpublished(ctx context.Context, id int64) (*entity.Post, error) {
	return r.selectPost(ctx, `
SELECT id, uuid, title, content, author_id, board_id, status, created_at, published_at, updated_at, deleted_at
FROM posts
WHERE id = ? AND status != 'deleted'
`, id)
}

func (r *PostRepository) SelectPostByUUIDIncludingUnpublished(ctx context.Context, postUUID string) (*entity.Post, error) {
	return r.selectPost(ctx, `
SELECT id, uuid, title, content, author_id, board_id, status, created_at, published_at, updated_at, deleted_at
FROM posts
WHERE uuid = ? AND status != 'deleted'
`, postUUID)
}

func (r *PostRepository) SelectPosts(ctx context.Context, boardID int64, limit int, lastID int64) ([]*entity.Post, error) {
	if limit <= 0 {
		return []*entity.Post{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, uuid, title, content, author_id, board_id, status, created_at, published_at, updated_at, deleted_at
FROM posts
WHERE board_id = ? AND status = 'published' AND (? <= 0 OR id < ?)
ORDER BY id DESC
LIMIT ?
`, boardID, lastID, lastID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts := make([]*entity.Post, 0, limit)
	for rows.Next() {
		post, scanErr := scanPost(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *PostRepository) SelectPublishedPostsByTagName(ctx context.Context, tagName string, limit int, lastID int64) ([]*entity.Post, error) {
	if limit <= 0 {
		return []*entity.Post{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT p.id, p.uuid, p.title, p.content, p.author_id, p.board_id, p.status, p.created_at, p.published_at, p.updated_at, p.deleted_at
FROM posts p
JOIN post_tags pt ON pt.post_id = p.id AND pt.status = 'active'
JOIN tags t ON t.id = pt.tag_id AND t.name = ?
WHERE p.status = 'published' AND (? <= 0 OR p.id < ?)
GROUP BY p.id
ORDER BY p.id DESC
LIMIT ?
`, tagName, lastID, lastID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts := make([]*entity.Post, 0, limit)
	for rows.Next() {
		post, scanErr := scanPost(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *PostRepository) ExistsByBoardID(ctx context.Context, boardID int64) (bool, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT 1
FROM posts
WHERE board_id = ? AND status != 'deleted'
LIMIT 1
`, boardID)
	var found int64
	err := row.Scan(&found)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, err
	}
}

func (r *PostRepository) ExistsByAuthorID(ctx context.Context, authorID int64) (bool, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT 1
FROM posts
WHERE author_id = ? AND status = 'published'
LIMIT 1
`, authorID)
	var found int64
	err := row.Scan(&found)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, err
	}
}

func (r *PostRepository) ExistsByAuthorIDIncludingDeleted(ctx context.Context, authorID int64) (bool, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT 1
FROM posts
WHERE author_id = ?
LIMIT 1
`, authorID)
	var found int64
	err := row.Scan(&found)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, err
	}
}

func (r *PostRepository) Update(ctx context.Context, post *entity.Post) error {
	if r == nil || r.db == nil {
		return errors.New("sqlite post repository is not initialized")
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE posts
SET uuid = ?, title = ?, content = ?, author_id = ?, board_id = ?, status = ?, created_at = ?, published_at = ?, updated_at = ?, deleted_at = ?
WHERE id = ?
`, post.UUID, post.Title, post.Content, post.AuthorID, post.BoardID, post.Status, post.CreatedAt.UnixNano(), timePtrToUnixNano(post.PublishedAt), post.UpdatedAt.UnixNano(), timePtrToUnixNano(post.DeletedAt), post.ID)
	return err
}

func (r *PostRepository) Delete(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return errors.New("sqlite post repository is not initialized")
	}
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
UPDATE posts
SET status = 'deleted', updated_at = ?, deleted_at = ?
WHERE id = ?
`, now.UnixNano(), now.UnixNano(), id)
	return err
}

func (r *PostRepository) selectPost(ctx context.Context, query string, arg any) (*entity.Post, error) {
	row := r.db.QueryRowContext(ctx, query, arg)
	post, err := scanPost(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return post, nil
}

func (r *PostRepository) selectPostUUIDsByIDs(ctx context.Context, ids []int64, includeDeleted bool) (map[int64]string, error) {
	posts, err := r.selectPostsByIDs(ctx, ids, includeDeleted)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(posts))
	for id, post := range posts {
		out[id] = post.UUID
	}
	return out, nil
}

func (r *PostRepository) selectPostsByIDs(ctx context.Context, ids []int64, includeDeleted bool) (map[int64]*entity.Post, error) {
	out := make(map[int64]*entity.Post, len(ids))
	for _, id := range ids {
		post, err := r.selectPostByIDIncludingDeleted(ctx, id)
		if err != nil {
			return nil, err
		}
		if post == nil {
			continue
		}
		if !includeDeleted && post.Status == entity.PostStatusDeleted {
			continue
		}
		out[id] = post
	}
	return out, nil
}

func (r *PostRepository) selectPostByIDIncludingDeleted(ctx context.Context, id int64) (*entity.Post, error) {
	return r.selectPost(ctx, `
SELECT id, uuid, title, content, author_id, board_id, status, created_at, published_at, updated_at, deleted_at
FROM posts
WHERE id = ?
`, id)
}

func (r *PostRepository) selectPostByUUIDIncludingDeleted(ctx context.Context, postUUID string) (*entity.Post, error) {
	return r.selectPost(ctx, `
SELECT id, uuid, title, content, author_id, board_id, status, created_at, published_at, updated_at, deleted_at
FROM posts
WHERE uuid = ?
`, postUUID)
}
