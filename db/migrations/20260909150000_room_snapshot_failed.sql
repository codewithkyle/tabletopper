-- migrate:up
-- Where a snapshot this build could not read is kept.
--
-- A ROOM WHOSE SNAPSHOT WILL NOT DECODE STARTS FRESH, and until now the first
-- save after that overwrote the column -- so the only copy of the old table
-- was gone the moment anybody moved a pawn. That is the one way the snapshot
-- design could lose a session rather than save it: a rollback to a build
-- with an older schema, or a decode bug, was a deploy that threw every
-- affected room's table away with no way back.
--
-- THE BYTES GO HERE FIRST. The hub writes the blob it could not read into this
-- column before the room's first save touches the other one, so a forward
-- build -- or a person with a JSON editor -- can put the table back. It is a
-- column and not a table because there is at most one such blob per room and
-- the newest failure is the one anybody wants; an older one it replaces was a
-- room already lost once.
--
-- NULLABLE, unlike snapshot, because nothing reads it on a hot path and
-- "there was never a failure" is a different fact from an empty object. The
-- timestamp beside it says when, which is what tells a rollback's failures
-- apart from a bug's.
ALTER TABLE rooms
    ADD COLUMN snapshot_failed JSON NULL AFTER snapshot_at,
    ADD COLUMN snapshot_failed_at DATETIME NULL AFTER snapshot_failed;

-- migrate:down
ALTER TABLE rooms
    DROP COLUMN snapshot_failed_at,
    DROP COLUMN snapshot_failed;
