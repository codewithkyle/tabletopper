-- migrate:up
CREATE TABLE IF NOT EXISTS assets (
    id  VARBINARY(16) PRIMARY KEY NOT NULL,
    owner_id VARBINARY(16) NOT NULL,

    file_path VARCHAR(1024) NOT NULL, -- Cloudflare R2 path relative to the user's directory
    type TINYINT UNSIGNED NOT NULL DEFAULT 0, -- 0:map, 1:avatar, 2:token, 3:music
    file_name VARCHAR(512) NOT NULL, -- the uploaded file name
    name VARCHAR(255) NOT NULL, -- custom label/name; default to file_name

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    KEY idx_assets_owner (owner_id),
    KEY idx_assets_owner_type (owner_id, type),
    CONSTRAINT chk_assets_type CHECK (type IN (0,1,2,3))
);

-- migrate:down
DROP TABLE IF EXISTS assets;
