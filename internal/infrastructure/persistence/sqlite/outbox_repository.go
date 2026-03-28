package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.OutboxStore = (*OutboxRepository)(nil)

type OutboxRepository struct {
	db                *sql.DB
	processingTimeout time.Duration
}

type outboxAppender struct {
	exec sqlExecutor
}

type outboxRepositoryOption func(*outboxRepositoryConfig)

type outboxRepositoryConfig struct {
	processingTimeout time.Duration
}

func WithProcessingTimeout(timeout time.Duration) outboxRepositoryOption {
	return func(cfg *outboxRepositoryConfig) {
		if timeout > 0 {
			cfg.processingTimeout = timeout
		}
	}
}

func NewOutboxRepository(db *sql.DB, opts ...outboxRepositoryOption) *OutboxRepository {
	cfg := outboxRepositoryConfig{
		processingTimeout: 30 * time.Second,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return &OutboxRepository{
		db:                db,
		processingTimeout: cfg.processingTimeout,
	}
}

func NewOutboxAppender(exec sqlExecutor) port.OutboxAppender {
	return &outboxAppender{exec: exec}
}

func normalizeOutboxContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func (r *OutboxRepository) Append(ctx context.Context, messages ...port.OutboxMessage) error {
	if r == nil || r.db == nil {
		return errors.New("sqlite outbox repository is not initialized")
	}
	return appendOutboxMessages(normalizeOutboxContext(ctx), r.db, messages...)
}

func (r *OutboxRepository) FetchReady(ctx context.Context, limit int, now time.Time) ([]port.OutboxMessage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("sqlite outbox repository is not initialized")
	}
	if limit <= 0 {
		return []port.OutboxMessage{}, nil
	}
	ctx = normalizeOutboxContext(ctx)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin sqlite outbox fetch transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	rows, err := tx.QueryContext(ctx, `
SELECT sequence, id, event_name, payload, occurred_at, attempt_count, next_attempt_at, status, last_error
FROM outbox_messages
WHERE status IN ('pending', 'processing') AND next_attempt_at <= ?
ORDER BY sequence ASC
LIMIT ?
`, now.UnixNano(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]port.OutboxMessage, 0, limit)
	for rows.Next() {
		item, err := scanOutboxMessage(rows)
		if err != nil {
			return nil, err
		}
		item.Status = port.OutboxStatusProcessing
		item.AttemptCount++
		item.NextAttemptAt = now.Add(r.processingTimeout)
		if _, err := tx.ExecContext(ctx, `
UPDATE outbox_messages
SET status = 'processing', attempt_count = ?, next_attempt_at = ?
WHERE sequence = ?
`, item.AttemptCount, item.NextAttemptAt.UnixNano(), item.Sequence); err != nil {
			return nil, err
		}
		messages = append(messages, item.OutboxMessage)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit sqlite outbox fetch transaction: %w", err)
	}
	return messages, nil
}

func (r *OutboxRepository) SelectByID(ctx context.Context, id string) (*port.OutboxMessage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("sqlite outbox repository is not initialized")
	}
	row := r.db.QueryRowContext(normalizeOutboxContext(ctx), `
SELECT sequence, id, event_name, payload, occurred_at, attempt_count, next_attempt_at, status, last_error
FROM outbox_messages
WHERE id = ?
LIMIT 1
`, id)
	message, err := scanOutboxMessage(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	out := message.OutboxMessage
	return &out, nil
}

func (r *OutboxRepository) SelectDead(ctx context.Context, limit int, lastID string) ([]port.OutboxMessage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("sqlite outbox repository is not initialized")
	}
	if limit <= 0 {
		return []port.OutboxMessage{}, nil
	}
	rows, err := r.db.QueryContext(normalizeOutboxContext(ctx), `
SELECT sequence, id, event_name, payload, occurred_at, attempt_count, next_attempt_at, status, last_error
FROM outbox_messages
WHERE status = 'dead'
ORDER BY occurred_at DESC, id DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]port.OutboxMessage, 0, limit)
	pastCursor := strings.TrimSpace(lastID) == ""
	for rows.Next() {
		item, err := scanOutboxMessage(rows)
		if err != nil {
			return nil, err
		}
		if !pastCursor {
			if item.ID == lastID {
				pastCursor = true
			}
			continue
		}
		items = append(items, item.OutboxMessage)
		if len(items) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *OutboxRepository) RenewProcessing(ctx context.Context, id string, nextAttemptAt time.Time) error {
	if r == nil || r.db == nil {
		return errors.New("sqlite outbox repository is not initialized")
	}
	_, err := r.db.ExecContext(normalizeOutboxContext(ctx), `
UPDATE outbox_messages
SET next_attempt_at = ?
WHERE id = ? AND status = 'processing'
`, nextAttemptAt.UnixNano(), id)
	return err
}

func (r *OutboxRepository) MarkSucceeded(ctx context.Context, ids ...string) error {
	if r == nil || r.db == nil {
		return errors.New("sqlite outbox repository is not initialized")
	}
	if len(ids) == 0 {
		return nil
	}
	ctx = normalizeOutboxContext(ctx)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite outbox success transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	for _, id := range ids {
		if strings.TrimSpace(id) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM outbox_messages WHERE id = ?`, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite outbox success transaction: %w", err)
	}
	return nil
}

func (r *OutboxRepository) MarkRetry(ctx context.Context, id string, nextAttemptAt time.Time, errMessage string) error {
	if r == nil || r.db == nil {
		return errors.New("sqlite outbox repository is not initialized")
	}
	_, err := r.db.ExecContext(normalizeOutboxContext(ctx), `
UPDATE outbox_messages
SET status = 'pending', next_attempt_at = ?, last_error = ?
WHERE id = ?
`, nextAttemptAt.UnixNano(), strings.TrimSpace(errMessage), id)
	return err
}

func (r *OutboxRepository) MarkDead(ctx context.Context, id string, errMessage string) error {
	if r == nil || r.db == nil {
		return errors.New("sqlite outbox repository is not initialized")
	}
	_, err := r.db.ExecContext(normalizeOutboxContext(ctx), `
UPDATE outbox_messages
SET status = 'dead', last_error = ?
WHERE id = ?
`, strings.TrimSpace(errMessage), id)
	return err
}

func (a *outboxAppender) Append(ctx context.Context, messages ...port.OutboxMessage) error {
	if a == nil || a.exec == nil {
		return errors.New("sqlite outbox appender is not initialized")
	}
	return appendOutboxMessages(normalizeOutboxContext(ctx), a.exec, messages...)
}

func appendOutboxMessages(ctx context.Context, exec sqlExecutor, messages ...port.OutboxMessage) error {
	now := time.Now().UTC()
	for _, message := range messages {
		if strings.TrimSpace(message.ID) == "" {
			continue
		}
		occurredAt := message.OccurredAt
		if occurredAt.IsZero() {
			occurredAt = now
		}
		nextAttemptAt := message.NextAttemptAt
		if nextAttemptAt.IsZero() {
			nextAttemptAt = occurredAt
		}
		status := message.Status
		if status == "" {
			status = port.OutboxStatusPending
		}
		payload := append([]byte(nil), message.Payload...)
		if payload == nil {
			payload = []byte{}
		}
		if _, err := exec.ExecContext(ctx, `
INSERT OR IGNORE INTO outbox_messages (
    id, event_name, payload, occurred_at, attempt_count, next_attempt_at, status, last_error
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, message.ID, message.EventName, payload, occurredAt.UnixNano(), message.AttemptCount, nextAttemptAt.UnixNano(), status, strings.TrimSpace(message.LastError)); err != nil {
			return err
		}
	}
	return nil
}

type outboxMessageRow struct {
	Sequence int64
	port.OutboxMessage
}

func scanOutboxMessage(scanner rowScanner) (outboxMessageRow, error) {
	var payload []byte
	var occurredAt sql.NullInt64
	var nextAttemptAt sql.NullInt64
	var attemptCount int
	var status string
	var lastError string
	item := outboxMessageRow{}
	if err := scanner.Scan(&item.Sequence, &item.ID, &item.EventName, &payload, &occurredAt, &attemptCount, &nextAttemptAt, &status, &lastError); err != nil {
		return outboxMessageRow{}, err
	}
	item.Payload = append([]byte(nil), payload...)
	item.OccurredAt = mustParseSQLTimestamp("outbox_messages.occurred_at", occurredAt)
	item.AttemptCount = attemptCount
	item.NextAttemptAt = mustParseSQLTimestamp("outbox_messages.next_attempt_at", nextAttemptAt)
	item.Status = port.OutboxStatus(status)
	item.LastError = lastError
	return item, nil
}
