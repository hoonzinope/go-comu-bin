package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.CommentRepository = (*CommentRepository)(nil)

type CommentRepository struct {
	exec sqlExecutor
}

func NewCommentRepository(exec sqlExecutor) *CommentRepository {
	return &CommentRepository{exec: exec}
}

func (r *CommentRepository) Save(ctx context.Context, comment *entity.Comment) (int64, error) {
	if r == nil || r.exec == nil {
		return 0, errors.New("sqlite comment repository is not initialized")
	}
	res, err := r.exec.ExecContext(ctx, `
INSERT INTO comments (
    uuid, content, author_id, post_id, parent_id, status, created_at, updated_at, deleted_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		comment.UUID,
		comment.Content,
		comment.AuthorID,
		comment.PostID,
		nullableInt64(comment.ParentID),
		string(comment.Status),
		comment.CreatedAt.UnixNano(),
		comment.UpdatedAt.UnixNano(),
		timePtrToUnixNano(comment.DeletedAt),
	)
	if err != nil {
		return 0, fmt.Errorf("save comment: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id for comment: %w", err)
	}
	comment.ID = id
	return id, nil
}

func (r *CommentRepository) SelectCommentByID(ctx context.Context, id int64) (*entity.Comment, error) {
	return r.selectComment(ctx, true, `
SELECT id, uuid, content, author_id, post_id, parent_id, status, created_at, updated_at, deleted_at
FROM comments
WHERE id = ? AND status = 'active'
LIMIT 1
`, id)
}

func (r *CommentRepository) SelectCommentByUUID(ctx context.Context, commentUUID string) (*entity.Comment, error) {
	return r.selectComment(ctx, false, `
SELECT id, uuid, content, author_id, post_id, parent_id, status, created_at, updated_at, deleted_at
FROM comments
WHERE uuid = ?
LIMIT 1
`, strings.TrimSpace(commentUUID))
}

func (r *CommentRepository) SelectCommentUUIDsByIDsIncludingDeleted(ctx context.Context, ids []int64) (map[int64]string, error) {
	if r == nil || r.exec == nil {
		return nil, errors.New("sqlite comment repository is not initialized")
	}
	if len(ids) == 0 {
		return map[int64]string{}, nil
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	rows, err := r.exec.QueryContext(ctx, fmt.Sprintf(`
SELECT id, uuid
FROM comments
WHERE id IN (%s)
`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return nil, fmt.Errorf("select comment uuids by ids: %w", err)
	}
	defer rows.Close()
	out := make(map[int64]string, len(ids))
	for rows.Next() {
		var id int64
		var commentUUID string
		if err := rows.Scan(&id, &commentUUID); err != nil {
			return nil, fmt.Errorf("select comment uuids by ids: %w", err)
		}
		out[id] = commentUUID
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("select comment uuids by ids: %w", err)
	}
	return out, nil
}

func (r *CommentRepository) SelectComments(ctx context.Context, postID int64, limit int, lastID int64) ([]*entity.Comment, error) {
	comments, err := r.selectActiveCommentsByPost(ctx, postID, limit, lastID)
	if err != nil {
		return nil, err
	}
	return comments, nil
}

func (r *CommentRepository) SelectCommentsIncludingDeleted(ctx context.Context, postID int64) ([]*entity.Comment, error) {
	return r.selectCommentsByPost(ctx, postID)
}

func (r *CommentRepository) SelectVisibleComments(ctx context.Context, postID int64, limit int, lastID int64) ([]*entity.Comment, error) {
	comments, err := r.selectCommentsByPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	filtered := filterVisibleComments(comments, lastID)
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

func (r *CommentRepository) ExistsByAuthorID(ctx context.Context, authorID int64) (bool, error) {
	return r.existsByAuthorID(ctx, authorID, true)
}

func (r *CommentRepository) ExistsByAuthorIDIncludingDeleted(ctx context.Context, authorID int64) (bool, error) {
	return r.existsByAuthorID(ctx, authorID, false)
}

func (r *CommentRepository) Update(ctx context.Context, comment *entity.Comment) error {
	if r == nil || r.exec == nil {
		return errors.New("sqlite comment repository is not initialized")
	}
	_, err := r.exec.ExecContext(ctx, `
UPDATE comments SET
    uuid = ?,
    content = ?,
    author_id = ?,
    post_id = ?,
    parent_id = ?,
    status = ?,
    created_at = ?,
    updated_at = ?,
    deleted_at = ?
WHERE id = ?
`,
		comment.UUID,
		comment.Content,
		comment.AuthorID,
		comment.PostID,
		nullableInt64(comment.ParentID),
		string(comment.Status),
		comment.CreatedAt.UnixNano(),
		comment.UpdatedAt.UnixNano(),
		timePtrToUnixNano(comment.DeletedAt),
		comment.ID,
	)
	if err != nil {
		return fmt.Errorf("update comment: %w", err)
	}
	return nil
}

func (r *CommentRepository) Delete(ctx context.Context, id int64) error {
	if r == nil || r.exec == nil {
		return errors.New("sqlite comment repository is not initialized")
	}
	now := time.Now().UTC()
	_, err := r.exec.ExecContext(ctx, `
UPDATE comments SET
    content = ?,
    status = 'deleted',
    updated_at = ?,
    deleted_at = ?
WHERE id = ?
`, entity.DeletedCommentPlaceholder, now.UnixNano(), now.UnixNano(), id)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	return nil
}

func (r *CommentRepository) selectComment(ctx context.Context, activeOnly bool, query string, args ...any) (*entity.Comment, error) {
	if r == nil || r.exec == nil {
		return nil, errors.New("sqlite comment repository is not initialized")
	}
	row := r.exec.QueryRowContext(ctx, query, args...)
	comment, err := scanComment(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select comment: %w", err)
	}
	if activeOnly && comment.Status != entity.CommentStatusActive {
		return nil, nil
	}
	return comment, nil
}

func (r *CommentRepository) selectCommentsByPost(ctx context.Context, postID int64) ([]*entity.Comment, error) {
	if r == nil || r.exec == nil {
		return nil, errors.New("sqlite comment repository is not initialized")
	}
	rows, err := r.exec.QueryContext(ctx, `
SELECT id, uuid, content, author_id, post_id, parent_id, status, created_at, updated_at, deleted_at
FROM comments
WHERE post_id = ?
ORDER BY id DESC
`, postID)
	if err != nil {
		return nil, fmt.Errorf("select comments by post: %w", err)
	}
	defer rows.Close()
	comments := make([]*entity.Comment, 0)
	for rows.Next() {
		comment, scanErr := scanComment(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("select comments by post: %w", scanErr)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("select comments by post: %w", err)
	}
	return comments, nil
}

func (r *CommentRepository) selectActiveCommentsByPost(ctx context.Context, postID int64, limit int, lastID int64) ([]*entity.Comment, error) {
	if r == nil || r.exec == nil {
		return nil, errors.New("sqlite comment repository is not initialized")
	}
	if limit <= 0 {
		return []*entity.Comment{}, nil
	}
	query := `
SELECT id, uuid, content, author_id, post_id, parent_id, status, created_at, updated_at, deleted_at
FROM comments
WHERE post_id = ?
  AND status = 'active'
`
	args := []any{postID}
	if lastID > 0 {
		query += "  AND id < ?\n"
		args = append(args, lastID)
	}
	query += `
ORDER BY id DESC
LIMIT ?
`
	args = append(args, limit)
	rows, err := r.exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select comments by post: %w", err)
	}
	defer rows.Close()
	comments := make([]*entity.Comment, 0, limit)
	for rows.Next() {
		comment, scanErr := scanComment(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("select comments by post: %w", scanErr)
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("select comments by post: %w", err)
	}
	return comments, nil
}

func (r *CommentRepository) existsByAuthorID(ctx context.Context, authorID int64, activeOnly bool) (bool, error) {
	if r == nil || r.exec == nil {
		return false, errors.New("sqlite comment repository is not initialized")
	}
	query := `SELECT 1 FROM comments WHERE author_id = ?`
	if activeOnly {
		query += ` AND status = 'active'`
	}
	query += ` LIMIT 1`
	row := r.exec.QueryRowContext(ctx, query, authorID)
	var found int
	if err := row.Scan(&found); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("exists comment by author: %w", err)
	}
	return true, nil
}

func filterVisibleComments(comments []*entity.Comment, lastID int64) []*entity.Comment {
	activeChildParentIDs := make(map[int64]struct{})
	for _, comment := range comments {
		if comment.Status == entity.CommentStatusActive && comment.ParentID != nil {
			activeChildParentIDs[*comment.ParentID] = struct{}{}
		}
	}

	filtered := make([]*entity.Comment, 0, len(comments))
	for _, comment := range comments {
		if lastID > 0 && comment.ID >= lastID {
			continue
		}
		if comment.Status == entity.CommentStatusActive {
			filtered = append(filtered, comment)
			continue
		}
		if _, ok := activeChildParentIDs[comment.ID]; ok {
			filtered = append(filtered, comment)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ID > filtered[j].ID
	})
	return filtered
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
