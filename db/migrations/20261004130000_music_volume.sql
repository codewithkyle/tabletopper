-- migrate:up
ALTER TABLE users
    ADD COLUMN music_volume TINYINT UNSIGNED NOT NULL DEFAULT 25 AFTER turn_notify;

-- migrate:down
ALTER TABLE users
    DROP COLUMN music_volume;
