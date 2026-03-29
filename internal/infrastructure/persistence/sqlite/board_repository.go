package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.BoardRepository = (*BoardRepository)(nil)

type BoardRepository struct {
	db sqlExecutor
}

func NewBoardRepository(db sqlExecutor) *BoardRepository {
	return &BoardRepository{db: db}
}

func (r *BoardRepository) Save(ctx context.Context, board *entity.Board) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("sqlite board repository is not initialized")
	}
	res, err := r.db.ExecContext(ctx, `
INSERT INTO boards (uuid, name, description, hidden, created_at)
VALUES (?, ?, ?, ?, ?)
`, board.UUID, board.Name, board.Description, boolToInt(board.Hidden), board.CreatedAt.UnixNano())
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	board.ID = id
	return id, nil
}

func (r *BoardRepository) SelectBoardByID(ctx context.Context, id int64) (*entity.Board, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("sqlite board repository is not initialized")
	}
	return r.selectBoard(ctx, `SELECT id, uuid, name, description, hidden, created_at FROM boards WHERE id = ?`, id)
}

func (r *BoardRepository) SelectBoardByUUID(ctx context.Context, boardUUID string) (*entity.Board, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("sqlite board repository is not initialized")
	}
	return r.selectBoard(ctx, `SELECT id, uuid, name, description, hidden, created_at FROM boards WHERE uuid = ?`, boardUUID)
}

func (r *BoardRepository) SelectBoardsByIDs(ctx context.Context, ids []int64) (map[int64]*entity.Board, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("sqlite board repository is not initialized")
	}
	if len(ids) == 0 {
		return map[int64]*entity.Board{}, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := fmt.Sprintf(
		`SELECT id, uuid, name, description, hidden, created_at FROM boards WHERE id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	selected := make(map[int64]*entity.Board, len(ids))
	for rows.Next() {
		board, scanErr := scanBoard(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		selected[board.ID] = board
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return selected, nil
}

func (r *BoardRepository) SelectBoardList(ctx context.Context, limit int, lastID int64) ([]*entity.Board, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("sqlite board repository is not initialized")
	}
	if limit <= 0 {
		return []*entity.Board{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, uuid, name, description, hidden, created_at
FROM boards
WHERE hidden = 0 AND (? <= 0 OR id < ?)
ORDER BY id DESC
LIMIT ?
`, lastID, lastID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	boards := make([]*entity.Board, 0, limit)
	for rows.Next() {
		board, scanErr := scanBoard(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		boards = append(boards, board)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return boards, nil
}

func (r *BoardRepository) SelectBoardListIncludingHidden(ctx context.Context, limit int, lastID int64) ([]*entity.Board, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("sqlite board repository is not initialized")
	}
	if limit <= 0 {
		return []*entity.Board{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, uuid, name, description, hidden, created_at
FROM boards
WHERE (? <= 0 OR id < ?)
ORDER BY id DESC
LIMIT ?
`, lastID, lastID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	boards := make([]*entity.Board, 0, limit)
	for rows.Next() {
		board, scanErr := scanBoard(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		boards = append(boards, board)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return boards, nil
}

func (r *BoardRepository) Update(ctx context.Context, board *entity.Board) error {
	if r == nil || r.db == nil {
		return errors.New("sqlite board repository is not initialized")
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE boards
SET name = ?, description = ?, hidden = ?
WHERE id = ?
`, board.Name, board.Description, boolToInt(board.Hidden), board.ID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return nil
	}
	return nil
}

func (r *BoardRepository) Delete(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return errors.New("sqlite board repository is not initialized")
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM boards WHERE id = ?`, id)
	return err
}

func (r *BoardRepository) selectBoard(ctx context.Context, query string, arg any) (*entity.Board, error) {
	row := r.db.QueryRowContext(ctx, query, arg)
	board, err := scanBoard(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return board, nil
}
