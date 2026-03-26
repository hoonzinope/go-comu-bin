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

var _ port.ReactionRepository = (*ReactionRepository)(nil)

type ReactionRepository struct {
	exec sqlExecutor
}

func NewReactionRepository(exec sqlExecutor) *ReactionRepository {
	return &ReactionRepository{exec: exec}
}

func (r *ReactionRepository) SetUserTargetReaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (*entity.Reaction, bool, bool, error) {
	if r == nil || r.exec == nil {
		return nil, false, false, errors.New("sqlite reaction repository is not initialized")
	}
	existing, err := r.GetUserTargetReaction(ctx, userID, targetID, targetType)
	if err != nil {
		return nil, false, false, err
	}
	if existing != nil {
		if existing.Type == reactionType {
			return existing, false, false, nil
		}
		updated := cloneReaction(existing)
		updated.Update(reactionType)
		_, err := r.exec.ExecContext(ctx, `
UPDATE reactions
SET type = ?
WHERE id = ?
`, updated.Type, updated.ID)
		if err != nil {
			return nil, false, false, fmt.Errorf("update reaction: %w", err)
		}
		return updated, false, true, nil
	}
	reaction := entity.NewReaction(targetType, targetID, reactionType, userID)
	res, err := r.exec.ExecContext(ctx, `
INSERT INTO reactions (
    target_type, target_id, type, user_id, created_at
) VALUES (?, ?, ?, ?, ?)
`, reaction.TargetType, reaction.TargetID, reaction.Type, reaction.UserID, reaction.CreatedAt.UnixNano())
	if err != nil {
		if uniqueConstraintError(err) {
			// Re-read to keep behavior stable under races.
			reloaded, readErr := r.GetUserTargetReaction(ctx, userID, targetID, targetType)
			if readErr != nil {
				return nil, false, false, readErr
			}
			if reloaded != nil {
				if reloaded.Type == reactionType {
					return reloaded, false, false, nil
				}
				reloaded.Update(reactionType)
				if _, err := r.exec.ExecContext(ctx, `
UPDATE reactions
SET type = ?
WHERE id = ?
`, reloaded.Type, reloaded.ID); err != nil {
					return nil, false, false, fmt.Errorf("update reaction after conflict: %w", err)
				}
				return reloaded, false, true, nil
			}
		}
		return nil, false, false, fmt.Errorf("save reaction: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, false, false, fmt.Errorf("last insert id for reaction: %w", err)
	}
	reaction.ID = id
	return reaction, true, true, nil
}

func (r *ReactionRepository) DeleteUserTargetReaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType) (bool, error) {
	if r == nil || r.exec == nil {
		return false, errors.New("sqlite reaction repository is not initialized")
	}
	res, err := r.exec.ExecContext(ctx, `
DELETE FROM reactions
WHERE user_id = ? AND target_id = ? AND target_type = ?
`, userID, targetID, targetType)
	if err != nil {
		return false, fmt.Errorf("delete reaction: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected for reaction delete: %w", err)
	}
	return affected > 0, nil
}

func (r *ReactionRepository) DeleteByTarget(ctx context.Context, targetID int64, targetType entity.ReactionTargetType) (int, error) {
	if r == nil || r.exec == nil {
		return 0, errors.New("sqlite reaction repository is not initialized")
	}
	res, err := r.exec.ExecContext(ctx, `
DELETE FROM reactions
WHERE target_id = ? AND target_type = ?
`, targetID, targetType)
	if err != nil {
		return 0, fmt.Errorf("delete reactions by target: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected for reaction target delete: %w", err)
	}
	return int(affected), nil
}

func (r *ReactionRepository) GetUserTargetReaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType) (*entity.Reaction, error) {
	if r == nil || r.exec == nil {
		return nil, errors.New("sqlite reaction repository is not initialized")
	}
	row := r.exec.QueryRowContext(ctx, `
SELECT id, target_type, target_id, type, user_id, created_at
FROM reactions
WHERE user_id = ? AND target_id = ? AND target_type = ?
LIMIT 1
`, userID, targetID, targetType)
	reaction, err := scanReaction(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get reaction: %w", err)
	}
	return reaction, nil
}

func (r *ReactionRepository) GetByTarget(ctx context.Context, targetID int64, targetType entity.ReactionTargetType) ([]*entity.Reaction, error) {
	items, err := r.GetByTargets(ctx, []int64{targetID}, targetType)
	if err != nil {
		return nil, err
	}
	return items[targetID], nil
}

func (r *ReactionRepository) GetByTargets(ctx context.Context, targetIDs []int64, targetType entity.ReactionTargetType) (map[int64][]*entity.Reaction, error) {
	if r == nil || r.exec == nil {
		return nil, errors.New("sqlite reaction repository is not initialized")
	}
	if len(targetIDs) == 0 {
		return map[int64][]*entity.Reaction{}, nil
	}
	placeholders := make([]string, 0, len(targetIDs))
	args := make([]any, 0, len(targetIDs)+1)
	args = append(args, targetType)
	for _, targetID := range targetIDs {
		placeholders = append(placeholders, "?")
		args = append(args, targetID)
	}
	rows, err := r.exec.QueryContext(ctx, fmt.Sprintf(`
SELECT id, target_type, target_id, type, user_id, created_at
FROM reactions
WHERE target_type = ? AND target_id IN (%s)
ORDER BY id ASC
`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return nil, fmt.Errorf("get reactions by targets: %w", err)
	}
	defer rows.Close()
	out := make(map[int64][]*entity.Reaction, len(targetIDs))
	for rows.Next() {
		reaction, scanErr := scanReaction(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("get reactions by targets: %w", scanErr)
		}
		out[reaction.TargetID] = append(out[reaction.TargetID], reaction)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get reactions by targets: %w", err)
	}
	return out, nil
}

func (r *ReactionRepository) ExistsByUserID(ctx context.Context, userID int64) (bool, error) {
	if r == nil || r.exec == nil {
		return false, errors.New("sqlite reaction repository is not initialized")
	}
	row := r.exec.QueryRowContext(ctx, `SELECT 1 FROM reactions WHERE user_id = ? LIMIT 1`, userID)
	var found int
	if err := row.Scan(&found); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("exists reaction by user: %w", err)
	}
	return true, nil
}
