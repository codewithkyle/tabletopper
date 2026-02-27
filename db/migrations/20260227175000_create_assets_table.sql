-- migrate:up
CREATE TABLE IF NOT EXISTS assets (
    id  BINARY(16) PRIMARY KEY NOT NULL,
    owner_id BINARY(16) NOT NULL,

    file_path VARCHAR(1024) NOT NULL, -- Cloudflare R2 path relative to the user's directory
    type TINYINT UNSIGNED NOT NULL DEFAULT 0, -- 0: map, 1: character, 2: token

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY ux_assets_owner_path (owner_id, file_path),
    KEY idx_assets_owner (owner_id),
    KEY idx_assets_owner_type (owner_id, type),
    CONSTRAINT chk_assets_type CHECK (type IN (0,1,2))
);

-- migrate:down
DROP TABLE IF EXISTS assets;
