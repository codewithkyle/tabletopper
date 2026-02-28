-- migrate:up
CREATE TABLE IF NOT EXISTS sessions (
    id  VARBINARY(16) PRIMARY KEY,
    hash VARBINARY(32) NOT NULL,

    user_id VARBINARY(16) NOT NULL,
    character_id VARBINARY(16) NULL,
    room_id VARBINARY(16) NULL,
    username VARCHAR(128) NOT NULL,
    profile_image_url VARCHAR(512) NOT NULL DEFAULT '/images/default-avatar.webp',

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    refreshed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,

    UNIQUE KEY ux_sessions_hash (hash),
    KEY idx_sessions_user_id (user_id),
    KEY idx_sessions_expires_at (expires_at)
);

-- migrate:down
DROP TABLE IF EXISTS sessions;
