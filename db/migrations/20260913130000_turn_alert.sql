-- migrate:up
ALTER TABLE users
    ADD COLUMN turn_alert BOOLEAN NOT NULL DEFAULT 1 AFTER ping_volume,
    ADD COLUMN turn_volume TINYINT UNSIGNED NOT NULL DEFAULT 100 AFTER turn_alert,
    ADD COLUMN turn_notify BOOLEAN NOT NULL DEFAULT 0 AFTER turn_volume;

-- migrate:down
ALTER TABLE users
    DROP COLUMN turn_notify,
    DROP COLUMN turn_volume,
    DROP COLUMN turn_alert;
