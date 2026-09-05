-- migrate:up
-- ux_spells_character_name is a UNIQUE key on the same two columns, so this
-- index only cost writes.
ALTER TABLE spells DROP INDEX idx_spells_character_name;

-- migrate:down
ALTER TABLE spells ADD INDEX idx_spells_character_name (character_id, name);
