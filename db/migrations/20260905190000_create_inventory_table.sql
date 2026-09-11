-- migrate:up
CREATE TABLE IF NOT EXISTS inventory (
    id VARBINARY(16) NOT NULL PRIMARY KEY,
    owner_id VARBINARY(16) NOT NULL,
    character_id VARBINARY(16) NOT NULL,

    name VARCHAR(128) NOT NULL DEFAULT '',
    quantity INT UNSIGNED NOT NULL DEFAULT 1,
    `value` VARCHAR(64) NOT NULL DEFAULT '',
    weight DECIMAL(8,2) NOT NULL DEFAULT 0,
    equipped BOOLEAN NOT NULL DEFAULT 0,
    description TEXT NOT NULL DEFAULT (''),

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    KEY idx_inventory_character (character_id, id)
);

-- migrate:down
DROP TABLE IF EXISTS inventory;
