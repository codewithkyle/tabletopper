-- migrate:up
ALTER TABLE assets
    ADD COLUMN uploaded_at DATETIME NULL AFTER tiled_at,
    ADD INDEX idx_assets_pending_upload (type, uploaded_at, created_at);

-- migrate:down
ALTER TABLE assets
    DROP INDEX idx_assets_pending_upload,
    DROP COLUMN uploaded_at;
