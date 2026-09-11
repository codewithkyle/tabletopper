-- migrate:up
CREATE TABLE IF NOT EXISTS spell_slots (
    character_id VARBINARY(16) NOT NULL,
    owner_id VARBINARY(16) NOT NULL,
    level TINYINT UNSIGNED NOT NULL,

    slots TINYINT UNSIGNED NOT NULL DEFAULT 0,
    used TINYINT UNSIGNED NOT NULL DEFAULT 0,

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (character_id, level),
    KEY idx_spell_slots_owner (owner_id),

    CONSTRAINT chk_spell_slots_level CHECK (level <= 9)
);

-- migrate:down
DROP TABLE IF EXISTS spell_slots;
