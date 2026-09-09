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
(id, owner_id, file_path, type, file_name, name, size_bytes)
VALUES (?, ?, ?, 'character', ?, ?, ?);

-- An account's own picture, which is `profile` and deliberately not `avatar`.
-- The Avatars page lists `type = 'avatar'` with no exception clause and offers a
-- Delete on every row it shows; a profile picture listed there would be
-- spawnable as an NPC's face and deletable in one click, and the delete would
-- leave users.avatar_asset_id pointing at nothing. See 20260909120000.
-- name: InsertProfilePicture :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name, size_bytes)
VALUES (?, ?, ?, 'profile', ?, ?, ?);

-- A monster's picture, and it is its own type rather than a token: `token` is
-- for one-off images placed on a map that belong to no monster, and this one is
-- what the manual card, the editor bar and eventually the pawn are all drawn
-- from. It is stored at 256 pixels rather than the avatar's 96, because a
-- portrait is a thumbnail and a pawn is drawn on a map at whatever zoom the GM
-- is at.
-- name: InsertMonsterImage :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name, size_bytes)
VALUES (?, ?, ?, 'monster', ?, ?, ?);

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
-- size_bytes is the multipart part's declared length, which is the original's
-- length and not the pyramid's. A tiled map costs the bucket its original plus
-- every tile of the generation that is serving, and the tiles are not counted:
-- they are derived, they are rebuilt from the original whenever the tiler runs
-- again, and there is no request that knows their total. What this column
-- measures is what an account uploaded.
-- name: InsertMap :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name, tile_size, size_bytes)
VALUES (?, ?, ?, 'map', ?, ?, ?, ?);

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
-- `profile` is here for the same reason and is the newest member: an account's
-- own picture is drawn on its pawn, in the player list, and beside the welcome
-- on its homepage, so it is at least as public as a portrait.
--
-- ADDING A MEMBER TO THE ENUM IS NOT ADDING IT HERE, and this list is where
-- that gets forgotten. A type absent from it reads as an asset that does not
-- exist: the row misses, the route answers 404, the object sits in the bucket,
-- and nothing logs. `profile` shipped that way and the symptom was a broken
-- picture on the homepage.
-- type comes back because a map is the one member here whose file_path must
-- never be served: it is the original PNG or JPEG, up to 128 MiB, and the only
-- picture a map has at an image route is the preview the tiler builds. See
-- serveImage, which refuses the case.
-- name: GetImage :one
SELECT id, type, file_path, preview_path, updated_at FROM assets
WHERE id = ? AND type IN ('map', 'avatar', 'token', 'monster', 'character', 'profile');

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

-- THE MAPS A ROOM CAN ACTUALLY USE, which is a smaller set than GetMaps. A map
-- with no tile_gen has no pyramid in the bucket yet -- it is queued, running,
-- or it failed -- and putting one on a layer would give every client at the
-- table a URL that 404s at every zoom.
--
-- IT IS THE LAYER MANAGER'S, which needs the name of the map each layer already
-- carries. The picker below is the one that offers a choice, and it deliberately
-- offers a wider set than this: a map that is still building belongs in front of
-- the person who has just uploaded it.
--
-- name: ListReadyMaps :many
SELECT id, name, width, height FROM assets
WHERE owner_id = ? AND type = 'map' AND tile_gen IS NOT NULL
ORDER BY name;

-- THE MAP PICKER'S TWO STATEMENTS: the whole shelf, and what a search matched.
-- The columns and the ordering are identical, because a filtered picker and an
-- unfiltered one are the same picker with fewer cards in it.
--
-- tile_attempts RIDES ALONG WITH tile_state because a failure is not one thing:
-- RequeueFailedTilingJobs comes back for a row that has failed fewer than three
-- times, so a card has to be able to say "trying again shortly" rather than
-- "gave up" to a map the worker has not finished with.
--
-- EVERY MAP IS HERE, tiled or not, which is what makes uploading from inside the
-- picker work. A map three seconds old has no pyramid and cannot be chosen, but
-- leaving it out would mean a GM presses Upload and nothing happens for a minute
-- -- so it comes back with its tile_state, the card says what it is doing, and
-- it turns into a button of its own accord when the tiles land. Choosing one
-- that is not ready is refused in resolveMap either way; the card not being a
-- button is the courtesy, not the rule.
--
-- UNFINISHED MAPS SORT FIRST, then everything alphabetically. `tile_gen IS NOT
-- NULL` is 0 for a map with no pyramid and 1 for a map with one, so ascending
-- puts the ones wanting attention at the top -- the upload that is building, and
-- anything that gave up -- and a GM watching a map tile does not have to know
-- what letter it starts with to find it. The rest is by name for the reason
-- above: a shelf that reorders itself is one that has to be read every time.
--
-- name: ListPickerMaps :many
SELECT id, name, file_name, width, height, tile_gen, tile_state, tile_attempts FROM assets
WHERE owner_id = ? AND type = 'map'
ORDER BY tile_gen IS NOT NULL, name;

-- BOTH NAMES ARE SEARCHED HERE, unlike SearchMaps below, which matches the
-- owner's name for the map and not the file it came from. The manager's reason
-- for that stands -- a search for "keep" should not hit every .keep.png -- and
-- this is the case it does not cover: the picker is reached mid-session with a
-- map in mind, and the thing a GM remembers about a map they renamed a month ago
-- is as often the file they exported as the name they typed. The card shows the
-- file name whenever it differs, so a hit that matched on it can be seen to have
-- matched on something.
--
-- name: SearchPickerMaps :many
SELECT id, name, file_name, width, height, tile_gen, tile_state, tile_attempts FROM assets
WHERE owner_id = sqlc.arg(owner_id) AND type = 'map'
  AND (name LIKE sqlc.arg(term) OR file_name LIKE sqlc.arg(term))
ORDER BY tile_gen IS NOT NULL, name;

-- name: GetMaps :many
SELECT * FROM assets
WHERE owner_id = ? AND type = 'map'
ORDER BY created_at DESC;

-- THE SEARCH BOX ON EACH OF THE FOUR MANAGER PAGES, one statement per page.
-- Each is its list statement above with a LIKE bolted on, and each keeps that
-- statement's columns and ordering, so a filtered page and an unfiltered one
-- are the same page with fewer cards in it.
--
-- THE TERM IS A PATTERN, NOT A WORD, and the caller escapes it: `%` and `_` are
-- wildcards to LIKE and ordinary characters to somebody typing, so an
-- unescaped `%` matches the whole shelf. The manual and the journal search
-- share the helper that does it.
--
-- ONLY name IS SEARCHED, NOT file_name. The chip on the card shows the file the
-- asset came from, but the name is the thing the owner typed and the thing they
-- will type again to find it; matching both would turn a search for "keep" into
-- a hit on every .keep.png somebody uploaded.
--
-- The rows are the owner's own and there are tens of them, so the LIKE scans
-- behind idx_assets_owner_type and reads nothing else. The table is
-- utf8mb4_0900_ai_ci, so it is already case- and accent-insensitive.
-- name: SearchMaps :many
SELECT * FROM assets
WHERE owner_id = sqlc.arg(owner_id) AND type = 'map' AND name LIKE sqlc.arg(term)
ORDER BY created_at DESC;

-- name: GetMap :one
SELECT * FROM assets
WHERE id = ? AND owner_id = ? AND type = 'map';

-- updated_at is set explicitly: ON UPDATE CURRENT_TIMESTAMP only fires when
-- a value changes, and re-uploading a file under its old name changes none.
-- The image proxy's ETag is built from updated_at, so it has to move.
--
-- size_bytes moves with it, because this is a replacement: the object at the
-- key was overwritten, so the length the row carries is the old picture's until
-- this writes the new one.
-- name: UpdateAssetFileName :exec
UPDATE assets
SET file_name = ?, size_bytes = ?, updated_at = NOW()
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
SET file_name = ?, size_bytes = ?, tile_state = 'pending', tile_attempts = 0
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
(id, owner_id, file_path, type, file_name, name, width, height, size_bytes)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetLibraryAssets :many
SELECT * FROM assets
WHERE owner_id = ? AND type = ?
ORDER BY created_at DESC;

-- The tokens and avatars boxes. The type is a parameter here for the reason it
-- is one above -- the two kinds differ in nothing but that word -- and it is
-- still never a value off the wire: the fragment route takes a kind, but it
-- matches it against the four members before it reaches any statement.
-- name: SearchLibraryAssets :many
SELECT * FROM assets
WHERE owner_id = sqlc.arg(owner_id) AND type = sqlc.arg(type) AND name LIKE sqlc.arg(term)
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
SET file_name = ?, width = ?, height = ?, size_bytes = ?, updated_at = NOW()
WHERE id = ? AND owner_id = ? AND type = ?;

-- MUSIC, WHICH IS THE ONE KIND WHOSE BYTES NEVER PASS THROUGH THIS PROCESS.
-- The browser PUTs them to R2 through a presigned URL, so an upload is two
-- requests with a gap between them and these statements are what carry the row
-- across it. See 20260907140000 for why uploaded_at exists.

-- The row that claims a key, written before the presigned URL that names it is
-- handed out. uploaded_at stays NULL, so nothing lists it and the sweep can
-- find it: it owns a key and nothing else until the confirm has looked in the
-- bucket.
-- name: InsertMusic :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name)
VALUES (?, ?, ?, 'music', ?, ?);

-- The library, which is finished tracks only. A row whose PUT is still running
-- -- or was abandoned when the tab closed -- has no object behind it, and a
-- card for it would be a player that answers every press with a 404.
-- name: GetMusicLibrary :many
SELECT * FROM assets
WHERE owner_id = ? AND type = 'music' AND uploaded_at IS NOT NULL
ORDER BY created_at DESC;

-- The music box. uploaded_at IS NOT NULL is carried over from the statement
-- above and is not optional: a search that dropped it would put a card for a
-- half-finished upload on the page the moment somebody typed a letter of its
-- name, and that card is a player answering every press with a 404.
-- name: SearchMusicLibrary :many
SELECT * FROM assets
WHERE owner_id = sqlc.arg(owner_id) AND type = 'music' AND uploaded_at IS NOT NULL
  AND name LIKE sqlc.arg(term)
ORDER BY created_at DESC;

-- One track, FINISHED OR NOT. The confirm reads a row that is by definition not
-- finished yet, so this one cannot filter on uploaded_at; every caller that
-- needs a playable track checks it for itself.
-- name: GetMusicTrack :one
SELECT * FROM assets
WHERE id = ? AND owner_id = ? AND type = 'music';

-- The second half of an upload: the object is in the bucket, it is the size the
-- signature allowed, and its first bytes say it is the format its name claims.
--
-- uploaded_at IS NULL IS IN THE WHERE, so a confirm that arrives twice writes
-- once. The result is read, because the second one landing on zero rows is how
-- the handler knows not to answer with a second card for a track the page
-- already shows.
-- size_bytes is written here rather than at the insert, and it is the only
-- kind that has to be: the row is created before the browser's PUT begins, so
-- at insert time the object does not exist and its length is whatever the
-- browser declared. The confirm's HEAD is the bucket's own answer, and it is
-- the number already checked against the cap two lines earlier in the handler.
-- name: FinishMusicUpload :execresult
UPDATE assets
SET uploaded_at = NOW(), size_bytes = ?
WHERE id = ? AND owner_id = ? AND type = 'music' AND uploaded_at IS NULL;

-- What one account is costing the bucket, across every kind at once.
--
-- IT IS A SUM AND NOT A COUNTER ON users. A counter would have to be kept in
-- step by every path that writes or deletes an asset, and those are the paths
-- that deliberately have no transaction around them: an upload writes the row
-- and then the object, and a delete removes the object and then the row,
-- because an object is not a row and cannot be rolled back with one. A counter
-- that drifted in one of those windows would be wrong permanently, with nothing
-- left to recompute it from. The sum is derived from the rows that ARE the
-- ledger, so it cannot disagree with them.
--
-- The index on owner_id covers it, and it is read once, when the settings
-- dialog is opened.
--
-- COALESCE for the account with no assets at all: SUM over no rows is NULL, and
-- a new account has to read 0 rather than nothing.
--
-- ROWS PREDATING 20260907150000 COUNT AS THE ZERO THEY WERE DEFAULTED TO, so
-- the total is what has been uploaded since. Nothing says so on the page: this
-- has only ever run against a local database, where the rows in question are a
-- handful of the developer's own and are replaced or deleted in the course of
-- using the app.
--
-- CAST, so this comes back as an integer rather than as sqlc's interface{}.
-- SUM over a BIGINT is DECIMAL in MySQL, which sqlc has no Go type for and
-- hands to the caller untyped; the cast is what makes the column a number the
-- handler can do arithmetic on without asserting.
-- name: SumOwnedAssetBytes :one
SELECT CAST(COALESCE(SUM(size_bytes), 0) AS SIGNED) AS total_bytes FROM assets
WHERE owner_id = ?;

-- The rows whose PUT never finished. A browser that was closed mid-upload
-- leaves one behind, and there is nothing in a request that could ever notice:
-- the request that would have confirmed it is the one that did not happen.
--
-- THE type = 'music' IS LOAD-BEARING. uploaded_at is NULL for every map, token,
-- avatar and journal image in the table, because those land inside the request
-- that inserted them and have no window for it to describe. Without this
-- clause, a pass would collect the entire assets table.
--
-- The grace is long enough to cover an upload that is merely slow rather than
-- abandoned -- see internal/sweep for the number and why.
--
-- The LIMIT is a literal because sqlc gives no parameter for one. It bounds a
-- pass rather than the backlog: each of these rows costs a delete against R2,
-- and what is left waits for the next pass.
-- name: ListAbandonedMusicUploads :many
SELECT id, owner_id, file_path FROM assets
WHERE type = 'music' AND uploaded_at IS NULL AND created_at < ?
ORDER BY created_at
LIMIT 100;
