CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL UNIQUE,
    email TEXT UNIQUE,
    password TEXT NOT NULL DEFAULT '',
    guest INTEGER NOT NULL DEFAULT 0,
    guest_status TEXT NOT NULL DEFAULT '',
    guest_issued_at INTEGER NULL,
    guest_activated_at INTEGER NULL,
    guest_expired_at INTEGER NULL,
    email_verified_at INTEGER NULL,
    role TEXT NOT NULL DEFAULT 'user',
    status TEXT NOT NULL DEFAULT 'active',
    suspension_reason TEXT NOT NULL DEFAULT '',
    suspended_until INTEGER NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    deleted_at INTEGER NULL
);

CREATE INDEX IF NOT EXISTS idx_users_status_guest ON users(status, guest, deleted_at);

CREATE TABLE IF NOT EXISTS email_verification_tokens (
    token_hash TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER NULL
);

CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_user_id ON email_verification_tokens(user_id);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    token_hash TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER NULL
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
