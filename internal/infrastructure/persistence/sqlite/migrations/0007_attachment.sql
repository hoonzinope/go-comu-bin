CREATE TABLE IF NOT EXISTS attachments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    post_id INTEGER NOT NULL,
    file_name TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    storage_key TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    orphaned_at INTEGER,
    pending_delete_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_attachments_post_id ON attachments(post_id);
CREATE INDEX IF NOT EXISTS idx_attachments_cleanup ON attachments(pending_delete_at, orphaned_at, id);
