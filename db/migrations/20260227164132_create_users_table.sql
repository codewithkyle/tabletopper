-- migrate:up
CREATE TABLE IF NOT EXISTS users (
    id  VARBINARY(16) PRIMARY KEY NOT NULL,
    clerk_id VARCHAR(255) NOT NULL,
    username VARCHAR(128) NOT NULL,
    profile_image_url VARCHAR(512) NOT NULL DEFAULT '/images/default-avatar.webp',

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE KEY ux_users_clerk_id (clerk_id)
);

-- migrate:down
DROP TABLE IF EXISTS users;
