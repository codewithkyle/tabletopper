-- migrate:up
ALTER TABLE users
    ADD COLUMN ping_volume TINYINT UNSIGNED NOT NULL DEFAULT 100 AFTER show_blood;

-- migrate:down
ALTER TABLE users
    DROP COLUMN ping_volume;
