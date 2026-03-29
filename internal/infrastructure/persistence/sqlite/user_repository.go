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
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.UserRepository = (*UserRepository)(nil)

type UserRepository struct {
	exec sqlExecutor
}

func NewUserRepository(exec sqlExecutor) *UserRepository {
	return &UserRepository{exec: exec}
}

func (r *UserRepository) Save(ctx context.Context, user *entity.User) (int64, error) {
	if r == nil || r.exec == nil {
		return 0, sqliteRepositoryUnavailableError("save user")
	}
	res, err := r.exec.ExecContext(ctx, `
INSERT INTO users (
    uuid, name, email, password, guest, guest_status,
    guest_issued_at, guest_activated_at, guest_expired_at, email_verified_at,
    role, status, suspension_reason, suspended_until, created_at, updated_at, deleted_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		user.UUID,
		user.Name,
		emailValueForStorage(user.Email),
		user.Password,
		boolToInt(user.Guest),
		string(user.GuestStatus),
		timePtrToUnixNano(user.GuestIssuedAt),
		timePtrToUnixNano(user.GuestActivatedAt),
		timePtrToUnixNano(user.GuestExpiredAt),
		timePtrToUnixNano(user.EmailVerifiedAt),
		user.Role,
		string(user.Status),
		user.SuspensionReason,
		timePtrToUnixNano(user.SuspendedUntil),
		user.CreatedAt.UnixNano(),
		user.UpdatedAt.UnixNano(),
		timePtrToUnixNano(user.DeletedAt),
	)
	if err != nil {
		if uniqueConstraintError(err) {
			return 0, customerror.ErrUserAlreadyExists
		}
		return 0, fmt.Errorf("save user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id for user: %w", err)
	}
	user.ID = id
	return id, nil
}

func (r *UserRepository) SelectUserByUsername(ctx context.Context, username string) (*entity.User, error) {
	return r.selectUser(ctx, "SELECT * FROM users WHERE name = ? AND deleted_at IS NULL LIMIT 1", strings.TrimSpace(username))
}

func (r *UserRepository) SelectUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	email = normalizeEmailForStorage(email)
	if email == "" {
		return nil, nil
	}
	return r.selectUser(ctx, "SELECT * FROM users WHERE email = ? AND deleted_at IS NULL LIMIT 1", email)
}

func (r *UserRepository) SelectUserByUUID(ctx context.Context, userUUID string) (*entity.User, error) {
	return r.selectUser(ctx, "SELECT * FROM users WHERE uuid = ? AND deleted_at IS NULL LIMIT 1", strings.TrimSpace(userUUID))
}

func (r *UserRepository) SelectUserByID(ctx context.Context, id int64) (*entity.User, error) {
	return r.selectUser(ctx, "SELECT * FROM users WHERE id = ? AND deleted_at IS NULL LIMIT 1", id)
}

func (r *UserRepository) SelectUserByIDIncludingDeleted(ctx context.Context, id int64) (*entity.User, error) {
	return r.selectUser(ctx, "SELECT * FROM users WHERE id = ? LIMIT 1", id)
}

func (r *UserRepository) SelectUsersByIDsIncludingDeleted(ctx context.Context, ids []int64) (map[int64]*entity.User, error) {
	if r == nil || r.exec == nil {
		return nil, sqliteRepositoryUnavailableError("select users by ids including deleted")
	}
	if len(ids) == 0 {
		return map[int64]*entity.User{}, nil
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	rows, err := r.exec.QueryContext(ctx, fmt.Sprintf("SELECT * FROM users WHERE id IN (%s)", strings.Join(placeholders, ",")), args...)
	if err != nil {
		return nil, fmt.Errorf("select users by ids: %w", err)
	}
	defer rows.Close()
	out := make(map[int64]*entity.User, len(ids))
	for rows.Next() {
		user, scanErr := scanUser(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		if user != nil {
			out[user.ID] = user
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("select users by ids: %w", err)
	}
	return out, nil
}

func (r *UserRepository) SelectGuestCleanupCandidates(ctx context.Context, now time.Time, pendingGrace, activeUnusedGrace time.Duration, limit int) ([]*entity.User, error) {
	if r == nil || r.exec == nil {
		return nil, sqliteRepositoryUnavailableError("select guest cleanup candidates")
	}
	if limit <= 0 {
		return []*entity.User{}, nil
	}
	rows, err := r.exec.QueryContext(ctx, `SELECT * FROM users WHERE guest = 1 AND deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("select guest cleanup candidates: %w", err)
	}
	defer rows.Close()
	candidates := make([]*entity.User, 0)
	for rows.Next() {
		user, scanErr := scanUser(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		if user == nil || !guestEligibleForCleanup(user, now, pendingGrace, activeUnusedGrace) {
			continue
		}
		candidates = append(candidates, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("select guest cleanup candidates: %w", err)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return guestCleanupEligibleAt(candidates[i]).Before(guestCleanupEligibleAt(candidates[j]))
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

func (r *UserRepository) Update(ctx context.Context, user *entity.User) error {
	if r == nil || r.exec == nil {
		return sqliteRepositoryUnavailableError("update user")
	}
	res, err := r.exec.ExecContext(ctx, `
UPDATE users SET
    uuid = ?,
    name = ?,
    email = ?,
    password = ?,
    guest = ?,
    guest_status = ?,
    guest_issued_at = ?,
    guest_activated_at = ?,
    guest_expired_at = ?,
    email_verified_at = ?,
    role = ?,
    status = ?,
    suspension_reason = ?,
    suspended_until = ?,
    created_at = ?,
    updated_at = ?,
    deleted_at = ?
WHERE id = ?
`,
		user.UUID,
		user.Name,
		emailValueForStorage(user.Email),
		user.Password,
		boolToInt(user.Guest),
		string(user.GuestStatus),
		timePtrToUnixNano(user.GuestIssuedAt),
		timePtrToUnixNano(user.GuestActivatedAt),
		timePtrToUnixNano(user.GuestExpiredAt),
		timePtrToUnixNano(user.EmailVerifiedAt),
		user.Role,
		string(user.Status),
		user.SuspensionReason,
		timePtrToUnixNano(user.SuspendedUntil),
		user.CreatedAt.UnixNano(),
		user.UpdatedAt.UnixNano(),
		timePtrToUnixNano(user.DeletedAt),
		user.ID,
	)
	if err != nil {
		if uniqueConstraintError(err) {
			return customerror.ErrUserAlreadyExists
		}
		return fmt.Errorf("update user: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected for user update: %w", err)
	}
	if affected == 0 {
		return nil
	}
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id int64) error {
	if r == nil || r.exec == nil {
		return sqliteRepositoryUnavailableError("delete user")
	}
	if _, err := r.exec.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func (r *UserRepository) selectUser(ctx context.Context, query string, args ...any) (*entity.User, error) {
	if r == nil || r.exec == nil {
		return nil, sqliteRepositoryUnavailableError("select user")
	}
	row := r.exec.QueryRowContext(ctx, query, args...)
	user, err := scanUserRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func sqliteRepositoryUnavailableError(op string) error {
	return customerror.WrapRepository(op, errors.New("sqlite user repository is not initialized"))
}

func scanUserRow(row scanner) (*entity.User, error) {
	var (
		id               int64
		uuid             string
		name             string
		email            sql.NullString
		password         string
		guest            int
		guestStatus      string
		guestIssuedAt    sql.NullInt64
		guestActivatedAt sql.NullInt64
		guestExpiredAt   sql.NullInt64
		emailVerifiedAt  sql.NullInt64
		role             string
		status           string
		suspensionReason string
		suspendedUntil   sql.NullInt64
		createdAt        int64
		updatedAt        int64
		deletedAt        sql.NullInt64
	)
	if err := row.Scan(
		&id,
		&uuid,
		&name,
		&email,
		&password,
		&guest,
		&guestStatus,
		&guestIssuedAt,
		&guestActivatedAt,
		&guestExpiredAt,
		&emailVerifiedAt,
		&role,
		&status,
		&suspensionReason,
		&suspendedUntil,
		&createdAt,
		&updatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	return &entity.User{
		ID:               id,
		UUID:             uuid,
		Name:             name,
		Email:            email.String,
		Password:         password,
		Guest:            guest != 0,
		GuestStatus:      entity.GuestStatus(guestStatus),
		GuestIssuedAt:    unixNanoToTimePtr(guestIssuedAt),
		GuestActivatedAt: unixNanoToTimePtr(guestActivatedAt),
		GuestExpiredAt:   unixNanoToTimePtr(guestExpiredAt),
		EmailVerifiedAt:  unixNanoToTimePtr(emailVerifiedAt),
		Role:             role,
		Status:           entity.UserStatus(status),
		SuspensionReason: suspensionReason,
		SuspendedUntil:   unixNanoToTimePtr(suspendedUntil),
		CreatedAt:        time.Unix(0, createdAt).UTC(),
		UpdatedAt:        time.Unix(0, updatedAt).UTC(),
		DeletedAt:        unixNanoToTimePtr(deletedAt),
	}, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(rows *sql.Rows) (*entity.User, error) {
	return scanUserRow(rows)
}

func normalizeEmailForStorage(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func emailValueForStorage(email string) any {
	normalized := normalizeEmailForStorage(email)
	if normalized == "" {
		return nil
	}
	return normalized
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func guestEligibleForCleanup(user *entity.User, now time.Time, pendingGrace, activeUnusedGrace time.Duration) bool {
	switch user.GuestStatus {
	case entity.GuestStatusPending:
		if user.GuestIssuedAt == nil {
			return false
		}
		return !user.GuestIssuedAt.Add(pendingGrace).After(now)
	case entity.GuestStatusExpired:
		if user.GuestExpiredAt == nil {
			return false
		}
		return !user.GuestExpiredAt.Add(pendingGrace).After(now)
	case entity.GuestStatusActive:
		basis := user.GuestActivatedAt
		if basis == nil {
			basis = user.GuestIssuedAt
		}
		if basis == nil {
			return false
		}
		return !basis.Add(activeUnusedGrace).After(now)
	default:
		return false
	}
}

func guestCleanupEligibleAt(user *entity.User) time.Time {
	switch user.GuestStatus {
	case entity.GuestStatusPending:
		if user.GuestIssuedAt != nil {
			return *user.GuestIssuedAt
		}
	case entity.GuestStatusExpired:
		if user.GuestExpiredAt != nil {
			return *user.GuestExpiredAt
		}
	case entity.GuestStatusActive:
		if user.GuestActivatedAt != nil {
			return *user.GuestActivatedAt
		}
		if user.GuestIssuedAt != nil {
			return *user.GuestIssuedAt
		}
	}
	return time.Time{}
}
