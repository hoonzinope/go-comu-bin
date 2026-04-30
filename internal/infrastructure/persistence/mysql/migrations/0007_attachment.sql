CREATE TABLE IF NOT EXISTS attachments (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid VARCHAR(36) NOT NULL UNIQUE,
    post_id BIGINT NOT NULL,
    file_name TEXT NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    size_bytes BIGINT NOT NULL,
    storage_key VARCHAR(255) NOT NULL,
    created_at BIGINT NOT NULL,
    orphaned_at BIGINT,
    pending_delete_at BIGINT
);

CREATE INDEX idx_attachments_post_id ON attachments(post_id);
CREATE INDEX idx_attachments_cleanup ON attachments(pending_delete_at, orphaned_at, id);
