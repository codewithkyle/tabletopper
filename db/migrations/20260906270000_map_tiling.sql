-- migrate:up
ALTER TABLE assets
    ADD COLUMN width INT UNSIGNED NULL AFTER detached_at,
    ADD COLUMN height INT UNSIGNED NULL AFTER width,
    ADD COLUMN tile_size SMALLINT UNSIGNED NULL AFTER height,
    ADD COLUMN max_zoom TINYINT UNSIGNED NULL AFTER tile_size,
    ADD COLUMN tile_gen VARBINARY(16) NULL AFTER max_zoom,
    ADD COLUMN tile_state ENUM('pending', 'working', 'ready', 'failed') NULL AFTER tile_gen,
    ADD COLUMN tile_attempts TINYINT UNSIGNED NOT NULL DEFAULT 0 AFTER tile_state,
    ADD COLUMN tile_lease VARBINARY(16) NULL AFTER tile_attempts,
    ADD COLUMN tile_leased_at DATETIME NULL AFTER tile_lease,
    ADD COLUMN tiled_at DATETIME NULL AFTER tile_leased_at,
    ADD INDEX idx_assets_tile_state (tile_state, created_at);

-- migrate:down
ALTER TABLE assets
    DROP INDEX idx_assets_tile_state,
    DROP COLUMN tiled_at,
    DROP COLUMN tile_leased_at,
    DROP COLUMN tile_lease,
    DROP COLUMN tile_attempts,
    DROP COLUMN tile_state,
    DROP COLUMN tile_gen,
    DROP COLUMN max_zoom,
    DROP COLUMN tile_size,
    DROP COLUMN height,
    DROP COLUMN width;
