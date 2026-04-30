CREATE TABLE IF NOT EXISTS outbox_messages (
    sequence BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    id VARCHAR(255) NOT NULL UNIQUE,
    event_name VARCHAR(255) NOT NULL,
    payload LONGBLOB NOT NULL,
    occurred_at BIGINT NOT NULL,
    attempt_count INT NOT NULL DEFAULT 0,
    next_attempt_at BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    last_error TEXT NOT NULL
);

CREATE INDEX idx_outbox_messages_status_next_attempt_sequence ON outbox_messages(status, next_attempt_at, sequence);
CREATE INDEX idx_outbox_messages_dead_occurred_id ON outbox_messages(status, occurred_at DESC, id DESC);
