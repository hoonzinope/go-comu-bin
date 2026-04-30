CREATE TABLE IF NOT EXISTS users (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid VARCHAR(36) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) UNIQUE,
    password VARCHAR(255) NOT NULL DEFAULT '',
    guest TINYINT(1) NOT NULL DEFAULT 0,
    guest_status VARCHAR(32) NOT NULL DEFAULT '',
    guest_issued_at BIGINT NULL,
    guest_activated_at BIGINT NULL,
    guest_expired_at BIGINT NULL,
    email_verified_at BIGINT NULL,
    role VARCHAR(32) NOT NULL DEFAULT 'user',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    suspension_reason VARCHAR(255) NOT NULL DEFAULT '',
    suspended_until BIGINT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    deleted_at BIGINT NULL
);

CREATE INDEX idx_users_status_guest ON users(status, guest, deleted_at);

CREATE TABLE IF NOT EXISTS email_verification_tokens (
    token_hash VARCHAR(255) PRIMARY KEY,
    user_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    expires_at BIGINT NOT NULL,
    consumed_at BIGINT NULL,
    delivered_at BIGINT NULL
);

CREATE INDEX idx_email_verification_tokens_user_id ON email_verification_tokens(user_id);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    token_hash VARCHAR(255) PRIMARY KEY,
    user_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    expires_at BIGINT NOT NULL,
    consumed_at BIGINT NULL,
    delivered_at BIGINT NULL
);

CREATE INDEX idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
