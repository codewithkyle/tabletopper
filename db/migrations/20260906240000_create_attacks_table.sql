-- migrate:up
CREATE TABLE IF NOT EXISTS attacks (
    id VARBINARY(16) NOT NULL PRIMARY KEY,
    owner_id VARBINARY(16) NOT NULL,
    character_id VARBINARY(16) NOT NULL,

    name VARCHAR(128) NOT NULL DEFAULT '',
    attack_bonus VARCHAR(32) NOT NULL DEFAULT '',
    damage VARCHAR(64) NOT NULL DEFAULT '',
    damage_type VARCHAR(32) NOT NULL DEFAULT '',
    mastery VARCHAR(32) NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT (''),

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    KEY idx_attacks_character (character_id, id)
);

-- migrate:down
DROP TABLE IF EXISTS attacks;
