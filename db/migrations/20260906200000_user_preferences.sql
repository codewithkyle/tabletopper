-- migrate:up
ALTER TABLE users
    ADD COLUMN theme ENUM('system', 'light', 'dark') NOT NULL DEFAULT 'system',
    ADD COLUMN timezone VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'America/New_York',
    ADD COLUMN date_format ENUM('dmy_text', 'mdy_text', 'mdy_slash', 'dmy_slash', 'iso') NOT NULL DEFAULT 'dmy_text',
    ADD COLUMN time_format ENUM('12h', '24h') NOT NULL DEFAULT '12h';

-- migrate:down
ALTER TABLE users
    DROP COLUMN time_format,
    DROP COLUMN date_format,
    DROP COLUMN timezone,
    DROP COLUMN theme;
