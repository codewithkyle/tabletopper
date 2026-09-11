-- migrate:up
ALTER TABLE characters DROP COLUMN notes;

-- migrate:down
ALTER TABLE characters ADD COLUMN notes TEXT NOT NULL DEFAULT ('');
