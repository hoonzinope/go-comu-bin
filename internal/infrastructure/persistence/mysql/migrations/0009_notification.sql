CREATE TABLE IF NOT EXISTS notifications (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid VARCHAR(36) NOT NULL UNIQUE,
    recipient_user_id BIGINT NOT NULL,
    actor_user_id BIGINT NOT NULL,
    type VARCHAR(32) NOT NULL,
    post_id BIGINT NOT NULL,
    comment_id BIGINT NOT NULL,
    actor_name_snapshot TEXT NOT NULL,
    post_title_snapshot TEXT NOT NULL,
    comment_preview_snapshot TEXT NOT NULL,
    read_at BIGINT,
    created_at BIGINT NOT NULL,
    dedup_key VARCHAR(255) NULL,
    UNIQUE KEY uq_notifications_dedup_key (dedup_key)
);

CREATE INDEX idx_notifications_recipient_id_id ON notifications(recipient_user_id, id DESC);
CREATE INDEX idx_notifications_unread ON notifications(recipient_user_id, read_at, id DESC);
