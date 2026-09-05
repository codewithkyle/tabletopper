-- migrate:up
-- Inventory is one entity per item, not a JSON blob on the character. Every
-- other repeater on the sheet posts its whole panel and the server replaces the
-- column, which works because a row's identity is its position in the list.
-- These rows are referenced from a second view -- the Character tab renders the
-- equipped ones -- so position is not enough, and a delete-all-rewrite save
-- would churn primary keys on every debounce. Each row is its own form instead,
-- and it needs an id to post to.
--
-- owner_id is denormalised the way spells does it, so a single-row write filters
-- on id, character_id and owner_id together with no join. All three, because the
-- item id and the character id both arrive in the URL and neither is trusted.
--
-- No `position` column. ULIDs sort lexicographically by creation time, so
-- ORDER BY id is insertion order and idx_inventory_character serves it.
-- Alphabetical would be worse than useless here: a freshly added blank row would
-- slide around the list as its name was typed.
--
-- EVERY COLUMN HAS A DEFAULT, description included -- TEXT cannot take a literal
-- default but takes an expression one. That is what lets the insert name three
-- columns and nothing else, so adding an item cannot carry item data the way a
-- wide insert could.
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
