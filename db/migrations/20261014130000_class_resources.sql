-- migrate:up
ALTER TABLE characters
    ADD COLUMN resources JSON NOT NULL DEFAULT (json_array());

-- migrate:down
ALTER TABLE characters
    DROP COLUMN resources;
