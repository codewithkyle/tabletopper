-- migrate:up
-- Slots and used are per level, not per spell, so they have nowhere to live in
-- the spells table. This is that nowhere.
--
-- Keyed on (character_id, level) rather than a ULID, because the level IS the
-- identity -- there is exactly one row per character per level and no way to
-- create a second. That makes the write an upsert instead of an insert, which
-- is the one thing in this rework with no precedent in the schema.
--
-- Rows are created on demand, not seeded ten at a time when a character is
-- created. Seeding would put ten inserts and a transaction behind
-- CreateCharacterFromName, which is a three-parameter statement on purpose, and
-- it would leave every character that has never cast a spell carrying ten rows
-- of zeroes. The read fills the gaps instead: ten levels always render, and a
-- level with no row renders as the zeroes it would have held.
--
-- owner_id is denormalised the way spells and inventory do it, so a single-row
-- write filters on character_id and owner_id together with no join.
--
-- TINYINT UNSIGNED caps a counter at 255. The old JSON path clamped to 99 in Go
-- and the input still says max="99"; the column is not the place that argues
-- about how many level-1 slots a character can have, it is the place that
-- refuses a number that would not fit.
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
