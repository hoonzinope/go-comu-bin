CREATE TABLE IF NOT EXISTS reactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_type TEXT NOT NULL,
    target_id INTEGER NOT NULL,
    type TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    UNIQUE(user_id, target_id, target_type)
);

CREATE INDEX IF NOT EXISTS idx_reactions_target_type_target_id ON reactions(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_reactions_user_id ON reactions(user_id);
