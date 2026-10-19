-- migrate:up
ALTER TABLE attacks
    ADD COLUMN sort_order INT UNSIGNED NOT NULL DEFAULT 0 AFTER notes,
    DROP INDEX idx_attacks_character,
    ADD INDEX idx_attacks_character (character_id, sort_order, id);

UPDATE attacks
JOIN (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY character_id ORDER BY created_at DESC, id DESC) - 1 AS place
    FROM attacks
) AS ranked ON ranked.id = attacks.id
SET attacks.sort_order = ranked.place;

ALTER TABLE spells
    ADD COLUMN sort_order INT UNSIGNED NOT NULL DEFAULT 0 AFTER is_prepared,
    DROP INDEX idx_spells_character_level,
    ADD INDEX idx_spells_character_level (character_id, level, sort_order, id);

UPDATE spells
JOIN (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY character_id, level ORDER BY created_at DESC, id DESC) - 1 AS place
    FROM spells
) AS ranked ON ranked.id = spells.id
SET spells.sort_order = ranked.place;

-- migrate:down
ALTER TABLE spells
    DROP INDEX idx_spells_character_level,
    ADD INDEX idx_spells_character_level (character_id, level, id),
    DROP COLUMN sort_order;

ALTER TABLE attacks
    DROP INDEX idx_attacks_character,
    ADD INDEX idx_attacks_character (character_id, id),
    DROP COLUMN sort_order;
