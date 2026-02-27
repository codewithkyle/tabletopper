-- migrate:up
CREATE TABLE IF NOT EXISTS spells (
    id BINARY(16) NOT NULL PRIMARY KEY,
    owner_id BINARY(16) NOT NULL,
    character_id BINARY(16) NOT NULL,

    level TINYINT UNSIGNED NOT NULL, -- 0..9 (cantrips = 0)
    name VARCHAR(128) NOT NULL,
    description VARCHAR(1024) NOT NULL DEFAULT '',
    is_prepared TINYINT(1) NOT NULL DEFAULT 0,

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY ux_spells_character_name (character_id, name),
    KEY idx_spells_character_level (character_id, level),
    KEY idx_spells_character_name (character_id, name),

    CONSTRAINT chk_spells_level CHECK (level <= 9)
);

-- migrate:down
DROP TABLE IF EXISTS spells;
