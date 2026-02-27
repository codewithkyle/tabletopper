-- migrate:up
CREATE TABLE IF NOT EXISTS characters (
    id BINARY(16) NOT NULL PRIMARY KEY,
    owner_id BINARY(16) NOT NULL,
    asset_id BINARY(16) NULL,

    name VARCHAR(128) NOT NULL,

    level TINYINT UNSIGNED NOT NULL DEFAULT 1,
    xp INT UNSIGNED NOT NULL DEFAULT 0,
    race VARCHAR(64) NULL,
    background VARCHAR(64) NULL,
    alignment VARCHAR(32) NULL,
    classes VARCHAR(64) NULL,

    ac SMALLINT UNSIGNED NOT NULL DEFAULT 10,
    max_hp SMALLINT UNSIGNED NOT NULL DEFAULT 1,
    current_hp SMALLINT UNSIGNED NOT NULL DEFAULT 1,
    proficiency_bonus SMALLINT UNSIGNED NOT NULL DEFAULT 2,
    temp_hp SMALLINT UNSIGNED NOT NULL DEFAULT 0,
    speed VARCHAR(128) NOT NULL,
    initiative_bonus SMALLINT NOT NULL DEFAULT 0,
    spell_save_dc SMALLINT UNSIGNED NOT NULL DEFAULT 0,
    spell_atk_bonus SMALLINT NOT NULL DEFAULT 0,

    `str` TINYINT UNSIGNED NOT NULL,
    dex TINYINT UNSIGNED NOT NULL,
    `con` TINYINT UNSIGNED NOT NULL,
    `int` TINYINT UNSIGNED NOT NULL,
    wis TINYINT UNSIGNED NOT NULL,
    cha TINYINT UNSIGNED NOT NULL,

    languages VARCHAR(255) NOT NULL,
    proficiencies VARCHAR(255) NOT NULL,
    skills JSON NOT NULL,
    saving_throws JSON NOT NULL,
    features JSON NOT NULL,
    weapons JSON NOT NULL,
    spell_slots JSON NOT NULL,
    resources JSON NOT NULL,

    notes TEXT NOT NULL,

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    KEY idx_characters_owner (owner_id),
    KEY idx_characters_owner_name (owner_id, name)
);

-- migrate:down
DROP TABLE IF EXISTS characters;
