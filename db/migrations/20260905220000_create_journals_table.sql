-- migrate:up
CREATE TABLE IF NOT EXISTS journals (
    id VARBINARY(16) NOT NULL PRIMARY KEY,
    owner_id VARBINARY(16) NOT NULL,
    character_id VARBINARY(16) NOT NULL,

    title VARCHAR(255) NOT NULL DEFAULT '',
    body MEDIUMTEXT NOT NULL DEFAULT (''),

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    KEY idx_journals_character (character_id, id)
);

-- migrate:down
DROP TABLE IF EXISTS journals;
