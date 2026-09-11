-- migrate:up
ALTER TABLE characters DROP COLUMN spell_slots;

-- migrate:down
ALTER TABLE characters ADD COLUMN spell_slots JSON NOT NULL DEFAULT ('{}');
