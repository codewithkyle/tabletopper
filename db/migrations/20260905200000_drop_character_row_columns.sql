-- migrate:up
ALTER TABLE characters
    DROP COLUMN weapons,
    DROP COLUMN resources;

-- migrate:down
ALTER TABLE characters
    ADD COLUMN weapons JSON NOT NULL DEFAULT ('[]'),
    ADD COLUMN resources JSON NOT NULL DEFAULT ('[]');
