-- Every query here is scoped to the owner except GetImage, which is the one
-- read that deliberately is not: a map or token is shown to every player at
-- the table, so any signed-in user may fetch any image by id. Ownership
-- gates the writes.

-- name: InsertAvatar :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name)
VALUES (?, ?, ?, 'avatar', ?, ?);

-- A map is inserted with nothing but its original and the size to tile it at.
-- preview_path and every pyramid column stay NULL until the worker has built
-- something to point them at, and the row is born pending because the upload
-- request does not decode the image at all -- it stores the bytes it was given
-- and queues the work.
-- name: InsertMap :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name, tile_size, tile_state)
VALUES (?, ?, ?, 'map', ?, ?, ?, 'pending');

-- name: GetImage :one
SELECT id, file_path, preview_path, updated_at FROM assets
WHERE id = ? AND type IN ('map', 'avatar', 'token');

-- name: GetMaps :many
SELECT * FROM assets
WHERE owner_id = ? AND type = 'map'
ORDER BY created_at DESC;

-- name: GetMap :one
SELECT * FROM assets
WHERE id = ? AND owner_id = ? AND type = 'map';

-- updated_at is set explicitly: ON UPDATE CURRENT_TIMESTAMP only fires when
-- a value changes, and re-uploading a file under its old name changes none.
-- The image proxy's ETag is built from updated_at, so it has to move.
-- name: UpdateAssetFileName :exec
UPDATE assets
SET file_name = ?, updated_at = NOW()
WHERE id = ? AND owner_id = ?;

-- name: UpdateAssetName :exec
UPDATE assets
SET name = ?
WHERE id = ? AND owner_id = ?;

-- name: DeleteAsset :exec
DELETE FROM assets
WHERE id = ? AND owner_id = ?;

-- Replacing a map overwrites its original and queues a rebuild. NOTHING IS
-- DELETED HERE: tile_gen still names the generation that is serving, and it
-- keeps serving until the worker has a whole new one to put in its place. A
-- job that was already mid-build finds its claim gone when it tries to publish
-- and throws away what it made.
-- name: RequeueMapForTiling :execresult
UPDATE assets
SET file_name = ?, tile_state = 'pending', tile_attempts = 0
WHERE id = ? AND owner_id = ? AND type = 'map';

-- The owner asking for one more go at a map whose tiling gave up. The attempt
-- counter goes back to zero because a person pressed a button, which is the
-- one thing the automatic retry cannot know about; tile_state = 'failed' in the
-- WHERE is what stops a second press re-queueing a job that is already running.
-- name: RetryMapTiling :execresult
UPDATE assets
SET tile_state = 'pending', tile_attempts = 0
WHERE id = ? AND owner_id = ? AND type = 'map' AND tile_state = 'failed';

-- The tiling worker's seven statements. Every one of them names the lease it
-- expects to find, because the lease is the claim: an UPDATE that matches no
-- row is how a worker learns that the map it was tiling was re-queued
-- underneath it, and none of these may write over a claim that is not theirs.

-- MySQL has no RETURNING, so the claim is two statements. The UPDATE is the
-- lock -- it takes one row and stamps a token nobody else has, so two processes
-- racing produce one winner and one zero-row result -- and the SELECT is how
-- the winner finds what it just took. tile_state is in the SELECT's WHERE so
-- the read goes through idx_assets_tile_state rather than scanning for a token.
-- name: ClaimMapForTiling :execresult
UPDATE assets
SET tile_state = 'working', tile_lease = ?, tile_leased_at = NOW()
WHERE type = 'map' AND tile_state = 'pending'
ORDER BY created_at
LIMIT 1;

-- name: GetLeasedMap :one
SELECT * FROM assets
WHERE type = 'map' AND tile_state = 'working' AND tile_lease = ?;

-- The pyramid columns and the job columns move in one statement, so the row's
-- account of what is serving can never disagree with what is in the bucket.
--
-- tile_gen IS PASSED IN RATHER THAN SET FROM tile_lease. MySQL evaluates a SET
-- list left to right and a later assignment sees an earlier one, so
-- `tile_gen = tile_lease, tile_lease = NULL` does work -- and silently stops
-- working the day someone sorts the list alphabetically. The value is the same
-- one the WHERE matches on, so passing it costs a placeholder and nothing else.
--
-- tile_attempts goes back to zero because the count is about the pyramid that
-- is serving, and this one arrived. A map that failed twice before succeeding
-- starts its next replacement with a full budget.
-- name: CompleteMapTiling :execresult
UPDATE assets
SET tile_state = 'ready',
    tile_gen = ?,
    tile_lease = NULL,
    tile_leased_at = NULL,
    tile_attempts = 0,
    width = ?,
    height = ?,
    tile_size = ?,
    max_zoom = ?,
    preview_path = ?,
    tiled_at = NOW()
WHERE id = ? AND type = 'map' AND tile_state = 'working' AND tile_lease = ?;

-- tile_leased_at is set rather than cleared, because on a failed row it is what
-- the retry waits on: a job that failed on a transient R2 error should not be
-- tried again on the next tick and burn its whole budget in a minute.
-- name: FailMapTiling :execresult
UPDATE assets
SET tile_state = 'failed',
    tile_attempts = tile_attempts + 1,
    tile_lease = NULL,
    tile_leased_at = NOW()
WHERE id = ? AND type = 'map' AND tile_state = 'working' AND tile_lease = ?;

-- A row left working by a process that died is stranded with nothing to notice,
-- so these two put it back. The generation named by tile_lease is deleted first
-- and the row is returned to pending after, which is the same objects-first
-- order every other delete in this app uses: a prefix no row names is an orphan
-- nothing will ever find.
--
-- The LIMIT is a literal because sqlc gives no parameter for one. It bounds a
-- pass rather than the backlog -- each of these rows costs a list and a delete
-- against R2, and what is left waits for the next pass.
-- name: ListStrandedTilingJobs :many
SELECT id, owner_id, tile_lease FROM assets
WHERE type = 'map' AND tile_state = 'working' AND tile_leased_at < ?
ORDER BY tile_leased_at
LIMIT 100;

-- name: RequeueStrandedTilingJob :execresult
UPDATE assets
SET tile_state = 'pending', tile_lease = NULL, tile_leased_at = NULL
WHERE id = ? AND type = 'map' AND tile_state = 'working' AND tile_lease = ?;

-- A failed row owns nothing in the bucket -- the failure deleted its generation
-- before writing the state -- so this one is a single statement with no R2 work
-- behind it. The attempt cap is what stops a poison upload, and the timestamp is
-- what makes the retries spread out rather than all happen at once.
-- name: RequeueFailedTilingJobs :execresult
UPDATE assets
SET tile_state = 'pending', tile_lease = NULL, tile_leased_at = NULL
WHERE type = 'map' AND tile_state = 'failed' AND tile_attempts < ? AND tile_leased_at < ?;
