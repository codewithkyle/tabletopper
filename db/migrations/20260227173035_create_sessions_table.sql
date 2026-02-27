-- migrate:up
CREATE TABLE IF NOT EXISTS sessions (
    id  BINARY(16) PRIMARY KEY,
    hash BINARY(32) NOT NULL,
    user_id BINARY(16) NOT NULL,

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    refreshed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,

    character_id BINARY(16) NULL,
    room_id BINARY(16) NULL,

    UNIQUE KEY ux_sessions_hash (hash),
    KEY idx_sessions_user_id (user_id),
    KEY idx_sessions_expires_at (expires_at)
);

-- migrate:down
DROP TABLE IF EXISTS sessions;
