CREATE TABLE IF NOT EXISTS reactions (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    target_type VARCHAR(32) NOT NULL,
    target_id BIGINT NOT NULL,
    type VARCHAR(32) NOT NULL,
    user_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    UNIQUE KEY uq_reactions_user_target (user_id, target_id, target_type)
);

CREATE INDEX idx_reactions_target_type_target_id ON reactions(target_type, target_id);
CREATE INDEX idx_reactions_user_id ON reactions(user_id);
