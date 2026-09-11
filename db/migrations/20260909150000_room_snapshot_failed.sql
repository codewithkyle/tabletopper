-- migrate:up
ALTER TABLE rooms
    ADD COLUMN snapshot_failed JSON NULL AFTER snapshot_at,
    ADD COLUMN snapshot_failed_at DATETIME NULL AFTER snapshot_failed;

-- migrate:down
ALTER TABLE rooms
    DROP COLUMN snapshot_failed_at,
    DROP COLUMN snapshot_failed;
