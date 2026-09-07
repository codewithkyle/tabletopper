-- Every query here is scoped to the owner except GetImage and GetMapPyramid,
-- which deliberately are not: a map, a token or a monster is shown to every
-- player at the table, so any signed-in user may fetch any image, and any tile
-- of any map, by id. Ownership gates the writes.

-- A character's portrait. It is `character` and not `avatar` for the reason a
-- monster's picture is `monster` and not `token`: the member says what owns the
-- row. This one is owned by a sheet and is reached through characters.asset_id;
-- `avatar` is library stock the account gathers, listed and deleted by its own
-- page, and a page offering Delete over a row a character points at would break
-- that portrait with nothing to notice -- there is no foreign key here to stop
-- it. See 20260907120000 for the move.
-- name: InsertCharacterPortrait :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name)
VALUES (?, ?, ?, 'character', ?, ?);

-- A monster's picture, and it is its own type rather than a token: `token` is
-- for one-off images placed on a map that belong to no monster, and this one is
-- what the manual card, the editor bar and eventually the pawn are all drawn
-- from. It is stored at 256 pixels rather than the avatar's 96, because a
-- portrait is a thumbnail and a pawn is drawn on a map at whatever zoom the GM
-- is at.
-- name: InsertMonsterImage :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name)
VALUES (?, ?, ?, 'monster', ?, ?);

-- A map is inserted with nothing but its original and the size to tile it at.
-- preview_path and every pyramid column stay NULL until the worker has built
-- something to point them at, because the upload request does not decode the
-- image at all -- it stores the bytes it was given and queues the work.
--
-- IT IS BORN WITH NO tile_state, WHICH IS WHAT STOPS IT BEING A JOB YET. The
-- row is written before the original is uploaded, so that no object ever
-- exists under a key no column names; but the worker claims on
-- tile_state = 'pending' alone, and the object it would go and read is not
-- there until the PUT after this returns. A NULL state is a row that owns a
-- key and nothing more. QueueMapForTiling is what turns it into work, once
-- there is something to work on.
-- name: InsertMap :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name, tile_size)
VALUES (?, ?, ?, 'map', ?, ?, ?);

-- The second half of an upload: the original has landed, so the row becomes a
-- job. Nothing reads the result -- the id is a ULID made in the request that
-- is still running, so there is no one else who could have moved the row, and
-- no answer this could give that the caller would act on differently.
-- name: QueueMapForTiling :exec
UPDATE assets
SET tile_state = 'pending'
WHERE id = ? AND owner_id = ? AND type = 'map';

-- The IN list is the whole of this statement's access rule, and every member of
-- it is a picture any signed-in user may fetch by id. A monster's image is
-- among them for the reason a map is: it is shown to every player at the table
-- the moment its pawn is put down, and a table is not a list of owners.
--
-- `journal` IS DELIBERATELY NOT HERE. A journal image belongs to one entry of
-- one character's diary, it is reached through the share's own reader route,
-- and it is the one stored picture that is nobody else's business.
-- `character` is here because it is where a character's portrait went, and the
-- portrait was being served from this statement as `avatar` the day before. It
-- is a picture every player at the table sees, like the four beside it.
-- name: GetImage :one
SELECT id, file_path, preview_path, updated_at FROM assets
WHERE id = ? AND type IN ('map', 'avatar', 'token', 'monster', 'character');

-- Everything the tile route needs to decide whether a requested tile exists,
-- and where it is. The four numbers are the pyramid's whole shape: the level
-- count follows from max_zoom and the grid at each level follows from width,
-- height and tile_size.
--
-- IT NEVER READS tile_state. The job columns and the pyramid columns are
-- separate so that a map being re-tiled can go on serving the generation it
-- has: tile_gen names what is in the bucket right now, and a tile request is
-- answered from that and is indifferent to whether a worker is running.
--
-- owner_id comes back because it is a component of the object's key, and the
-- request that asks for a tile has no business supplying it.
-- name: GetMapPyramid :one
SELECT owner_id, width, height, tile_size, max_zoom, tile_gen FROM assets
WHERE id = ? AND type = 'map';

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

-- The type is in the WHERE so a rename cannot cross kinds. Every rename arrives
-- at /assets/<kind>/<id>/name, and without it the kind in that path would be
-- decoration: a token's id posted to the maps route would rename the token, and
-- the reply is a toast that says it worked.
-- name: UpdateAssetName :exec
UPDATE assets
SET name = ?
WHERE id = ? AND owner_id = ? AND type = ?;

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

-- THE LIBRARY, WHICH IS EVERY KIND THE ACCOUNT GATHERS RATHER THAN A SHEET
-- OWNS. Tokens and avatars today, music next; maps keep their own statements
-- because a map is a pyramid and a job as well as a row.
--
-- THE TYPE IS A PARAMETER HERE AND A LITERAL EVERYWHERE ELSE, and that is the
-- one thing about these worth arguing over. It is a parameter because the four
-- rows differ in nothing but that word -- same columns, same owner scoping,
-- same ordering -- so a statement per kind would be four copies to keep in
-- step. It is safe because no request ever supplies it: the handler is reached
-- through a route naming one kind, and passes the kind that route is for. A
-- value off the wire would have to be matched against an allowlist first, and
-- there is no route here that takes one.
--
-- Every one of them is scoped to the owner, unlike GetImage and GetMapPyramid.
-- Those serve pictures to a whole table; these are the manager, and the manager
-- shows an account its own shelf.

-- width and height are the STORED image's dimensions, not the upload's. A token
-- is fitted into a box with its aspect kept and an avatar is cropped square, so
-- what a renderer needs is what came out of the encoder -- and it needs it to
-- place a token at the right shape without fetching and decoding the file
-- first. They are written for every kind so that "the pixels of the image at
-- file_path" is true of a library row whatever kind it is.
-- name: InsertLibraryAsset :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name, width, height)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetLibraryAssets :many
SELECT * FROM assets
WHERE owner_id = ? AND type = ?
ORDER BY created_at DESC;

-- name: GetLibraryAsset :one
SELECT * FROM assets
WHERE id = ? AND owner_id = ? AND type = ?;

-- Replacing a library asset overwrites its object at the key it already has, so
-- the row keeps its id, its name and its path, and only what came out of the
-- new encode changes.
--
-- updated_at is set explicitly for the reason UpdateAssetFileName gives: ON
-- UPDATE CURRENT_TIMESTAMP fires only when a value changes, and replacing a
-- file with one of the same name and the same dimensions changes none of them.
-- The image proxy's ETag is built from updated_at, so a replacement that did
-- not move it would go on serving the old picture out of the browser's cache.
-- name: ReplaceLibraryAsset :exec
UPDATE assets
SET file_name = ?, width = ?, height = ?, updated_at = NOW()
WHERE id = ? AND owner_id = ? AND type = ?;
