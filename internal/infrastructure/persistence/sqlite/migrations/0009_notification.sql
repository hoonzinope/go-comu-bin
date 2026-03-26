CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    recipient_user_id INTEGER NOT NULL,
    actor_user_id INTEGER NOT NULL,
    type TEXT NOT NULL,
    post_id INTEGER NOT NULL,
    comment_id INTEGER NOT NULL,
    actor_name_snapshot TEXT NOT NULL,
    post_title_snapshot TEXT NOT NULL,
    comment_preview_snapshot TEXT NOT NULL,
    read_at INTEGER,
    created_at INTEGER NOT NULL,
    dedup_key TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_dedup_key ON notifications(dedup_key) WHERE dedup_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_notifications_recipient_id_id ON notifications(recipient_user_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications(recipient_user_id, read_at, id DESC);
