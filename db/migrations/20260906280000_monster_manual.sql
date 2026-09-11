-- migrate:up
DROP TABLE IF EXISTS monsters;

CREATE TABLE monsters (
    id VARBINARY(16) NOT NULL PRIMARY KEY,
    owner_id VARBINARY(16) NOT NULL,
    asset_id VARBINARY(16) NULL,

    name VARCHAR(128) NOT NULL,
    size VARCHAR(32) NOT NULL DEFAULT 'medium',
    type VARCHAR(32) NOT NULL DEFAULT 'humanoid',
    tags VARCHAR(128) NOT NULL DEFAULT '',
    alignment VARCHAR(32) NOT NULL DEFAULT 'unaligned',

    ac TINYINT UNSIGNED NOT NULL DEFAULT 10,
    hp SMALLINT UNSIGNED NOT NULL DEFAULT 1,
    hit_dice VARCHAR(64) NOT NULL DEFAULT '',
    speed VARCHAR(128) NOT NULL DEFAULT '30 ft.',
    initiative_bonus SMALLINT NOT NULL DEFAULT 0,
    cr VARCHAR(4) NOT NULL DEFAULT '0',
    legendary_action_uses TINYINT UNSIGNED NOT NULL DEFAULT 0,
    legendary_action_uses_in_lair TINYINT UNSIGNED NOT NULL DEFAULT 0,

    `str` TINYINT UNSIGNED NOT NULL DEFAULT 10,
    dex TINYINT UNSIGNED NOT NULL DEFAULT 10,
    `con` TINYINT UNSIGNED NOT NULL DEFAULT 10,
    `int` TINYINT UNSIGNED NOT NULL DEFAULT 10,
    wis TINYINT UNSIGNED NOT NULL DEFAULT 10,
    cha TINYINT UNSIGNED NOT NULL DEFAULT 10,

    skills JSON NOT NULL DEFAULT (JSON_OBJECT()),
    skill_proficiencies JSON NOT NULL DEFAULT (JSON_OBJECT()),
    saving_throws JSON NOT NULL DEFAULT (JSON_OBJECT()),
    saving_throw_proficiencies JSON NOT NULL DEFAULT (JSON_OBJECT()),

    vulnerabilities VARCHAR(512) NOT NULL DEFAULT '',
    resistances VARCHAR(512) NOT NULL DEFAULT '',
    immunities VARCHAR(512) NOT NULL DEFAULT '',
    gear VARCHAR(512) NOT NULL DEFAULT '',
    senses VARCHAR(255) NOT NULL DEFAULT '',
    languages VARCHAR(255) NOT NULL DEFAULT '',

    habitat VARCHAR(255) NOT NULL DEFAULT '',
    treasure VARCHAR(64) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT (''),

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    KEY idx_monsters_owner_name (owner_id, name)
);

CREATE TABLE monster_actions (
    id VARBINARY(16) NOT NULL PRIMARY KEY,
    owner_id VARBINARY(16) NOT NULL,
    monster_id VARBINARY(16) NOT NULL,
    kind ENUM(
        'trait', 'action', 'bonus_action', 'reaction',
        'legendary_action', 'lair_action', 'regional_effect'
    ) NOT NULL,

    name VARCHAR(128) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT (''),

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    KEY idx_monster_actions_monster (monster_id, kind, id)
);

ALTER TABLE assets
    MODIFY `type` ENUM('map', 'avatar', 'token', 'music', 'journal', 'monster') NOT NULL DEFAULT 'map';

-- migrate:down
DELETE FROM assets WHERE `type` = 'monster';

ALTER TABLE assets
    MODIFY `type` ENUM('map', 'avatar', 'token', 'music', 'journal') NOT NULL DEFAULT 'map';

DROP TABLE IF EXISTS monster_actions;

DROP TABLE IF EXISTS monsters;

CREATE TABLE monsters (
    id VARBINARY(16) NOT NULL PRIMARY KEY,
    owner_id VARBINARY(16) NOT NULL,
    asset_id VARBINARY(16) NOT NULL,

    name VARCHAR(255) NOT NULL,
    size VARCHAR(32) NOT NULL,
    type VARCHAR(64) NOT NULL,
    subtype VARCHAR(128) NOT NULL,
    alignment VARCHAR(64) NOT NULL,

    ac INT NOT NULL,
    hp INT NOT NULL,
    hit_dice VARCHAR(64) NOT NULL,

    `str` TINYINT UNSIGNED NOT NULL,
    dex TINYINT UNSIGNED NOT NULL,
    `con` TINYINT UNSIGNED NOT NULL,
    `int` TINYINT UNSIGNED NOT NULL,
    wis TINYINT UNSIGNED NOT NULL,
    cha TINYINT UNSIGNED NOT NULL,

    languages VARCHAR(255) NOT NULL,
    cr VARCHAR(4) NOT NULL,
    xp INT UNSIGNED NOT NULL,

    speed VARCHAR(128) NOT NULL,

    vulnerabilities VARCHAR(512) NOT NULL,
    resistances VARCHAR(512) NOT NULL,
    immunities VARCHAR(512) NOT NULL,
    senses VARCHAR(512) NOT NULL,

    saving_throws VARCHAR(512) NOT NULL,
    skills VARCHAR(512) NOT NULL,

    abilities JSON NOT NULL,
    actions JSON NOT NULL,
    bonus_actions JSON NOT NULL,
    legendary_actions JSON NOT NULL,
    reactions JSON NOT NULL,
    lair_actions JSON NOT NULL,

    KEY idx_monsters_owner (owner_id),
    KEY idx_monsters_owner_name (owner_id, name)
);
