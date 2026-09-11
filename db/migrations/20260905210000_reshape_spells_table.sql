-- migrate:up
ALTER TABLE spells DROP INDEX ux_spells_character_name;

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
