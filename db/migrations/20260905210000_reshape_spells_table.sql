-- migrate:up
-- The spells table has existed since the first schema and has never had a query
-- written against it. Spells live in characters.spell_slots, a JSON object
-- holding all ten levels at once, and the shape here was designed for a
-- different feature than the one the editor actually grew.
--
-- What the editor collects per spell is a name, a school, components, a casting
-- time, a range, a duration and the spell text. What this table has is `name`
-- and one `description`. The five columns below are the difference, and
-- description becomes the spell text it was always going to hold.
--
-- VARCHAR(1024) was too small for it either way -- that counts characters, and
-- the longer SRD spells run past it. TEXT takes an expression default, which is
-- what keeps every column defaulted.
--
-- EVERY COLUMN HAS A DEFAULT once this runs, name included. That is what lets
-- the insert name id, owner_id, character_id and level and nothing else, the
-- same property the inventory table has and for the same reason: a row is
-- created blank by a button press and named afterwards, so an add cannot carry
-- spell data even if a caller tried to supply some.
--
-- school defaults to Evocation because a select must have something selected and
-- that is what the blank card has always rendered. Go keeps its own
-- DefaultSpellSchool for a different job -- rejecting an invalid posted value --
-- so the two are not duplicates of each other.
--
-- casting_range, not `range`. RANGE is reserved in MySQL 8 and later, so the
-- column would need backticks in every statement that touched it.

-- ux_spells_character_name has to go. A spell row is created blank and named
-- afterwards, so the second Add Spell on a level would collide with the first on
-- (character_id, ''). It was also the wrong rule to enforce: nothing stops a
-- character carrying the same spell twice, and the sheet is not the place to
-- argue about it.
ALTER TABLE spells DROP INDEX ux_spells_character_name;

-- A level page reads WHERE character_id = ? AND level = ? ORDER BY id, so id
-- belongs in the index. Dropped and re-added rather than redefined in place:
-- MySQL only allows a same-name DROP and ADD inside one ALTER under
-- ALGORITHM=COPY.
ALTER TABLE spells DROP INDEX idx_spells_character_level;
ALTER TABLE spells ADD INDEX idx_spells_character_level (character_id, level, id);

ALTER TABLE spells
    MODIFY name VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN school VARCHAR(32) NOT NULL DEFAULT 'Evocation' AFTER name,
    ADD COLUMN components VARCHAR(128) NOT NULL DEFAULT '' AFTER school,
    ADD COLUMN casting_time VARCHAR(64) NOT NULL DEFAULT '' AFTER components,
    ADD COLUMN casting_range VARCHAR(64) NOT NULL DEFAULT '' AFTER casting_time,
    ADD COLUMN duration VARCHAR(64) NOT NULL DEFAULT '' AFTER casting_range,
    MODIFY description TEXT NOT NULL DEFAULT ('');

-- migrate:down
-- description comes back as VARCHAR(1024), which truncates anything longer in
-- strict mode rather than silently cutting it -- down restores the shape, and a
-- row that outgrew it does not fit through.
ALTER TABLE spells
    DROP COLUMN school,
    DROP COLUMN components,
    DROP COLUMN casting_time,
    DROP COLUMN casting_range,
    DROP COLUMN duration,
    MODIFY description VARCHAR(1024) NOT NULL DEFAULT '',
    MODIFY name VARCHAR(128) NOT NULL;

ALTER TABLE spells DROP INDEX idx_spells_character_level;
ALTER TABLE spells ADD INDEX idx_spells_character_level (character_id, level);
ALTER TABLE spells ADD UNIQUE KEY ux_spells_character_name (character_id, name);
