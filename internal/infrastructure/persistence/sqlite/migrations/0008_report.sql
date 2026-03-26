CREATE TABLE IF NOT EXISTS reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_type TEXT NOT NULL,
    target_id INTEGER NOT NULL,
    reporter_user_id INTEGER NOT NULL,
    reason_code TEXT NOT NULL,
    reason_detail TEXT NOT NULL,
    status TEXT NOT NULL,
    resolution_note TEXT NOT NULL DEFAULT '',
    resolved_by INTEGER,
    resolved_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(reporter_user_id, target_type, target_id)
);

CREATE INDEX IF NOT EXISTS idx_reports_status_id ON reports(status, id DESC);
CREATE INDEX IF NOT EXISTS idx_reports_reporter_user_id ON reports(reporter_user_id);
