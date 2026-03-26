package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.TagRepository = (*TagRepository)(nil)

type TagRepository struct {
	db sqlExecutor
}

func NewTagRepository(db sqlExecutor) *TagRepository {
	return &TagRepository{db: db}
}

func (r *TagRepository) Save(ctx context.Context, tag *entity.Tag) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("sqlite tag repository is not initialized")
	}
	res, err := r.db.ExecContext(ctx, `
INSERT OR IGNORE INTO tags (name, created_at)
VALUES (?, ?)
`, tag.Name, tag.CreatedAt.UnixNano())
	if err != nil {
		return 0, err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if rowsAffected == 0 {
		existing, err := r.selectByName(ctx, tag.Name)
		if err != nil {
			return 0, err
		}
		if existing == nil {
			return 0, errors.New("sqlite tag save conflict without existing row")
		}
		tag.ID = existing.ID
		return existing.ID, nil
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	tag.ID = id
	return id, nil
}

func (r *TagRepository) SelectByName(ctx context.Context, name string) (*entity.Tag, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("sqlite tag repository is not initialized")
	}
	return r.selectByName(ctx, name)
}

func (r *TagRepository) SelectByIDs(ctx context.Context, ids []int64) ([]*entity.Tag, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("sqlite tag repository is not initialized")
	}
	if len(ids) == 0 {
		return []*entity.Tag{}, nil
	}
	tags := make([]*entity.Tag, 0, len(ids))
	for _, id := range ids {
		tag, err := r.selectByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if tag == nil {
			continue
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func (r *TagRepository) selectByName(ctx context.Context, name string) (*entity.Tag, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, created_at FROM tags WHERE name = ?`, name)
	tag, err := scanTag(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return tag, nil
}

func (r *TagRepository) selectByID(ctx context.Context, id int64) (*entity.Tag, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, created_at FROM tags WHERE id = ?`, id)
	tag, err := scanTag(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return tag, nil
}
