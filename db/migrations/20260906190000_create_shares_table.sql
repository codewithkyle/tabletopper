-- migrate:up
CREATE TABLE shares (
    id VARBINARY(16) NOT NULL,
    owner_id VARBINARY(16) NOT NULL,
    character_id VARBINARY(16) NOT NULL,
    resource_type ENUM('journal') NOT NULL DEFAULT 'journal',
    resource_id VARBINARY(16) NOT NULL,
    token CHAR(22) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    password_hash VARCHAR(60) CHARACTER SET ascii COLLATE ascii_bin NULL,
    expires_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY ux_shares_token (token),
    UNIQUE KEY ux_shares_resource (resource_type, resource_id),
    KEY idx_shares_character (character_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- migrate:down
DROP TABLE shares;
