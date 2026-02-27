-- migrate:up
CREATE TABLE IF NOT EXISTS monsters (
    id BINARY(16) NOT NULL PRIMARY KEY,
    owner_id BINARY(16) NOT NULL,
    asset_id BINARY(16) NOT NULL,

    name VARCHAR(255) NOT NULL,
    size VARCHAR(32) NOT NULL,
    type VARCHAR(64) NOT NULL,
    subtype VARCHAR(128) NOT NULL,
    alignment VARCHAR(64) NOT NULL,

    ac INT NOT NULL,
    hp INT NOT NULL,
    hit_dice VARCHAR(64) NOT NULL,

    `str` INT NOT NULL,
    dex INT NOT NULL,
    `con` INT NOT NULL,
    `int` INT NOT NULL,
    wis INT NOT NULL,
    cha INT NOT NULL,

    languages VARCHAR(512) NOT NULL,
    cr INT NOT NULL,
    xp INT NOT NULL,

    speed VARCHAR(255) NOT NULL,

    vulnerabilities VARCHAR(512) NOT NULL,
    resistances VARCHAR(512) NOT NULL,
    immunities VARCHAR(512) NOT NULL,
    senses VARCHAR(512) NOT NULL,

    saving_throws VARCHAR(512) NOT NULL,
    skills VARCHAR(512) NOT NULL,

    abilities JSON NOT NULL,
    actions JSON NOT NULL,
    bonusActions JSON NOT NULL,
    legendaryActions JSON NOT NULL,
    reactions JSON NOT NULL,
    lairActions JSON NOT NULL,

    KEY idx_monsters_user_id (owner_id),
    KEY idx_monsters_owner_name (owner_id, name)
);

-- migrate:down
DROP TABLE IF EXISTS monsters;
