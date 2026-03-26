package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.EmailVerificationTokenRepository = (*EmailVerificationTokenRepository)(nil)
var _ port.PasswordResetTokenRepository = (*PasswordResetTokenRepository)(nil)

type EmailVerificationTokenRepository struct {
	exec sqlExecutor
}

type PasswordResetTokenRepository struct {
	exec sqlExecutor
}

func NewEmailVerificationTokenRepository(exec sqlExecutor) *EmailVerificationTokenRepository {
	return &EmailVerificationTokenRepository{exec: exec}
}

func NewPasswordResetTokenRepository(exec sqlExecutor) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{exec: exec}
}

func (r *EmailVerificationTokenRepository) Save(ctx context.Context, token *entity.EmailVerificationToken) error {
	return saveToken(ctx, r.exec, "email_verification_tokens", token.UserID, token.TokenHash, token.CreatedAt, token.ExpiresAt, token.ConsumedAt)
}

func (r *EmailVerificationTokenRepository) SelectByTokenHash(ctx context.Context, tokenHash string) (*entity.EmailVerificationToken, error) {
	return selectEmailVerificationToken(ctx, r.exec, tokenHash)
}

func (r *EmailVerificationTokenRepository) InvalidateByUser(ctx context.Context, userID int64) error {
	return invalidateTokensByUser(ctx, r.exec, "email_verification_tokens", userID)
}

func (r *EmailVerificationTokenRepository) Update(ctx context.Context, token *entity.EmailVerificationToken) error {
	return saveToken(ctx, r.exec, "email_verification_tokens", token.UserID, token.TokenHash, token.CreatedAt, token.ExpiresAt, token.ConsumedAt)
}

func (r *EmailVerificationTokenRepository) DeleteExpiredOrConsumedBefore(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	return deleteExpiredOrConsumedBefore(ctx, r.exec, "email_verification_tokens", cutoff, limit)
}

func (r *PasswordResetTokenRepository) Save(ctx context.Context, token *entity.PasswordResetToken) error {
	return saveToken(ctx, r.exec, "password_reset_tokens", token.UserID, token.TokenHash, token.CreatedAt, token.ExpiresAt, token.ConsumedAt)
}

func (r *PasswordResetTokenRepository) SelectByTokenHash(ctx context.Context, tokenHash string) (*entity.PasswordResetToken, error) {
	return selectPasswordResetToken(ctx, r.exec, tokenHash)
}

func (r *PasswordResetTokenRepository) InvalidateByUser(ctx context.Context, userID int64) error {
	return invalidateTokensByUser(ctx, r.exec, "password_reset_tokens", userID)
}

func (r *PasswordResetTokenRepository) Update(ctx context.Context, token *entity.PasswordResetToken) error {
	return saveToken(ctx, r.exec, "password_reset_tokens", token.UserID, token.TokenHash, token.CreatedAt, token.ExpiresAt, token.ConsumedAt)
}

func (r *PasswordResetTokenRepository) DeleteExpiredOrConsumedBefore(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	return deleteExpiredOrConsumedBefore(ctx, r.exec, "password_reset_tokens", cutoff, limit)
}

func saveToken(ctx context.Context, exec sqlExecutor, table string, userID int64, tokenHash string, createdAt, expiresAt time.Time, consumedAt *time.Time) error {
	if exec == nil {
		return fmt.Errorf("sqlite token repository is not initialized")
	}
	_, err := exec.ExecContext(ctx, fmt.Sprintf(`
INSERT INTO %s (
    token_hash, user_id, created_at, expires_at, consumed_at
) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(token_hash) DO UPDATE SET
    user_id = excluded.user_id,
    created_at = excluded.created_at,
    expires_at = excluded.expires_at,
    consumed_at = excluded.consumed_at
`, table),
		tokenHash,
		userID,
		createdAt.UnixNano(),
		expiresAt.UnixNano(),
		timePtrToUnixNano(consumedAt),
	)
	if err != nil {
		return fmt.Errorf("save token in %s: %w", table, err)
	}
	return nil
}

func invalidateTokensByUser(ctx context.Context, exec sqlExecutor, table string, userID int64) error {
	if exec == nil {
		return fmt.Errorf("sqlite token repository is not initialized")
	}
	_, err := exec.ExecContext(ctx, fmt.Sprintf(`
UPDATE %s
SET consumed_at = COALESCE(consumed_at, ?)
WHERE user_id = ? AND consumed_at IS NULL
`, table), time.Now().UnixNano(), userID)
	if err != nil {
		return fmt.Errorf("invalidate tokens in %s: %w", table, err)
	}
	return nil
}

func deleteExpiredOrConsumedBefore(ctx context.Context, exec sqlExecutor, table string, cutoff time.Time, limit int) (int, error) {
	if exec == nil {
		return 0, fmt.Errorf("sqlite token repository is not initialized")
	}
	if limit <= 0 {
		return 0, nil
	}
	rows, err := exec.QueryContext(ctx, fmt.Sprintf(`
SELECT token_hash
FROM %s
WHERE (consumed_at IS NOT NULL AND consumed_at <= ?)
   OR expires_at <= ?
ORDER BY COALESCE(consumed_at, expires_at), token_hash
LIMIT ?
`, table), cutoff.UnixNano(), cutoff.UnixNano(), limit)
	if err != nil {
		return 0, fmt.Errorf("select deletable tokens in %s: %w", table, err)
	}
	defer rows.Close()
	hashes := make([]string, 0)
	for rows.Next() {
		var tokenHash string
		if err := rows.Scan(&tokenHash); err != nil {
			return 0, fmt.Errorf("scan deletable token in %s: %w", table, err)
		}
		hashes = append(hashes, tokenHash)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("select deletable tokens in %s: %w", table, err)
	}
	for _, tokenHash := range hashes {
		if _, err := exec.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE token_hash = ?", table), tokenHash); err != nil {
			return 0, fmt.Errorf("delete token in %s: %w", table, err)
		}
	}
	return len(hashes), nil
}

func selectEmailVerificationToken(ctx context.Context, exec sqlExecutor, tokenHash string) (*entity.EmailVerificationToken, error) {
	row := exec.QueryRowContext(ctx, `
SELECT token_hash, user_id, created_at, expires_at, consumed_at
FROM email_verification_tokens
WHERE token_hash = ?
`, tokenHash)
	var (
		out        entity.EmailVerificationToken
		createdAt  int64
		expiresAt  int64
		consumedAt sql.NullInt64
	)
	if err := row.Scan(&out.TokenHash, &out.UserID, &createdAt, &expiresAt, &consumedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("select email verification token: %w", err)
	}
	out.CreatedAt = time.Unix(0, createdAt).UTC()
	out.ExpiresAt = time.Unix(0, expiresAt).UTC()
	out.ConsumedAt = unixNanoToTimePtr(consumedAt)
	return &out, nil
}

func selectPasswordResetToken(ctx context.Context, exec sqlExecutor, tokenHash string) (*entity.PasswordResetToken, error) {
	row := exec.QueryRowContext(ctx, `
SELECT token_hash, user_id, created_at, expires_at, consumed_at
FROM password_reset_tokens
WHERE token_hash = ?
`, tokenHash)
	var (
		out        entity.PasswordResetToken
		createdAt  int64
		expiresAt  int64
		consumedAt sql.NullInt64
	)
	if err := row.Scan(&out.TokenHash, &out.UserID, &createdAt, &expiresAt, &consumedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("select password reset token: %w", err)
	}
	out.CreatedAt = time.Unix(0, createdAt).UTC()
	out.ExpiresAt = time.Unix(0, expiresAt).UTC()
	out.ConsumedAt = unixNanoToTimePtr(consumedAt)
	return &out, nil
}
