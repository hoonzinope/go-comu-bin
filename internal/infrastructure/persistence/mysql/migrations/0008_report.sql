CREATE TABLE IF NOT EXISTS reports (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    target_type VARCHAR(32) NOT NULL,
    target_id BIGINT NOT NULL,
    reporter_user_id BIGINT NOT NULL,
    reason_code VARCHAR(255) NOT NULL,
    reason_detail LONGTEXT NOT NULL,
    status VARCHAR(32) NOT NULL,
    resolution_note TEXT NOT NULL,
    resolved_by BIGINT,
    resolved_at BIGINT,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    UNIQUE KEY uq_reports_reporter_target (reporter_user_id, target_type, target_id)
);

CREATE INDEX idx_reports_status_id ON reports(status, id DESC);
CREATE INDEX idx_reports_reporter_user_id ON reports(reporter_user_id);
