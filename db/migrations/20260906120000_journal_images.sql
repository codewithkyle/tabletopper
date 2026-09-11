-- migrate:up
ALTER TABLE assets
    MODIFY COLUMN type ENUM('map', 'avatar', 'token', 'music', 'journal') NOT NULL DEFAULT 'map',
    ADD COLUMN journal_id VARBINARY(16) NULL AFTER owner_id,
    ADD COLUMN detached_at DATETIME NULL AFTER name,
    ADD INDEX idx_assets_journal (journal_id);

-- migrate:down
DELETE FROM assets WHERE type = 'journal';
ALTER TABLE assets
    DROP INDEX idx_assets_journal,
    DROP COLUMN detached_at,
    DROP COLUMN journal_id,
    MODIFY COLUMN type ENUM('map', 'avatar', 'token', 'music') NOT NULL DEFAULT 'map';
