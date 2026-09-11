-- migrate:up
ALTER TABLE assets
    ADD COLUMN size_bytes BIGINT NOT NULL DEFAULT 0 AFTER file_name;

-- migrate:down
ALTER TABLE assets
    DROP COLUMN size_bytes;
