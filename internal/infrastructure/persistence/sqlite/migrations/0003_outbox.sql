CREATE TABLE IF NOT EXISTS outbox_messages (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    event_name TEXT NOT NULL,
    payload BLOB NOT NULL,
    occurred_at INTEGER NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    last_error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_outbox_messages_status_next_attempt_sequence ON outbox_messages(status, next_attempt_at, sequence);
CREATE INDEX IF NOT EXISTS idx_outbox_messages_dead_occurred_id ON outbox_messages(status, occurred_at DESC, id DESC);
