-- name: InsertCharacterPortrait :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name, size_bytes)
VALUES (?, ?, ?, 'character', ?, ?, ?);
-- name: InsertProfilePicture :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name, size_bytes)
VALUES (?, ?, ?, 'profile', ?, ?, ?);
-- name: InsertMonsterImage :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name, size_bytes)
VALUES (?, ?, ?, 'monster', ?, ?, ?);
-- name: InsertMap :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name, tile_size, size_bytes)
VALUES (?, ?, ?, 'map', ?, ?, ?, ?);
-- name: QueueMapForTiling :exec
UPDATE assets
SET tile_state = 'pending'
WHERE id = ? AND owner_id = ? AND type = 'map';
-- name: GetImage :one
SELECT id, type, file_path, preview_path, updated_at FROM assets
WHERE id = ? AND type IN ('map', 'avatar', 'token', 'monster', 'character', 'profile');
-- name: GetMapPyramid :one
SELECT owner_id, width, height, tile_size, max_zoom, tile_gen FROM assets
WHERE id = ? AND type = 'map';
-- name: ListReadyMaps :many
SELECT id, name, width, height FROM assets
WHERE owner_id = ? AND type = 'map' AND tile_gen IS NOT NULL
ORDER BY name;
-- name: ListPickerMaps :many
SELECT id, name, file_name, width, height, tile_gen, tile_state, tile_attempts FROM assets
WHERE owner_id = ? AND type = 'map'
ORDER BY tile_gen IS NOT NULL, name;
-- name: SearchPickerMaps :many
SELECT id, name, file_name, width, height, tile_gen, tile_state, tile_attempts FROM assets
WHERE owner_id = sqlc.arg(owner_id) AND type = 'map'
  AND (name LIKE sqlc.arg(term) OR file_name LIKE sqlc.arg(term))
ORDER BY tile_gen IS NOT NULL, name;
-- name: GetMaps :many
SELECT * FROM assets
WHERE owner_id = ? AND type = 'map'
ORDER BY created_at DESC;
-- name: SearchMaps :many
SELECT * FROM assets
WHERE owner_id = sqlc.arg(owner_id) AND type = 'map' AND name LIKE sqlc.arg(term)
ORDER BY created_at DESC;
-- name: GetMap :one
SELECT * FROM assets
WHERE id = ? AND owner_id = ? AND type = 'map';
-- name: UpdateAssetFileName :exec
UPDATE assets
SET file_name = ?, size_bytes = ?, updated_at = NOW()
WHERE id = ? AND owner_id = ?;
-- name: UpdateAssetName :exec
UPDATE assets
SET name = ?
WHERE id = ? AND owner_id = ? AND type = ?;
-- name: DeleteAsset :exec
DELETE FROM assets
WHERE id = ? AND owner_id = ?;
-- name: RequeueMapForTiling :execresult
UPDATE assets
SET file_name = ?, size_bytes = ?, tile_state = 'pending', tile_attempts = 0
WHERE id = ? AND owner_id = ? AND type = 'map';
-- name: RetryMapTiling :execresult
UPDATE assets
SET tile_state = 'pending', tile_attempts = 0
WHERE id = ? AND owner_id = ? AND type = 'map' AND tile_state = 'failed';
-- name: ClaimMapForTiling :execresult
UPDATE assets
SET tile_state = 'working', tile_lease = ?, tile_leased_at = NOW()
WHERE type = 'map' AND tile_state = 'pending'
ORDER BY created_at
LIMIT 1;
-- name: GetLeasedMap :one
SELECT * FROM assets
WHERE type = 'map' AND tile_state = 'working' AND tile_lease = ?;
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
-- name: FailMapTiling :execresult
UPDATE assets
SET tile_state = 'failed',
    tile_attempts = tile_attempts + 1,
    tile_lease = NULL,
    tile_leased_at = NOW()
WHERE id = ? AND type = 'map' AND tile_state = 'working' AND tile_lease = ?;
-- name: ListStrandedTilingJobs :many
SELECT id, owner_id, tile_lease FROM assets
WHERE type = 'map' AND tile_state = 'working' AND tile_leased_at < ?
ORDER BY tile_leased_at
LIMIT 100;
-- name: RequeueStrandedTilingJob :execresult
UPDATE assets
SET tile_state = 'pending', tile_lease = NULL, tile_leased_at = NULL
WHERE id = ? AND type = 'map' AND tile_state = 'working' AND tile_lease = ?;
-- name: RequeueFailedTilingJobs :execresult
UPDATE assets
SET tile_state = 'pending', tile_lease = NULL, tile_leased_at = NULL
WHERE type = 'map' AND tile_state = 'failed' AND tile_attempts < ? AND tile_leased_at < ?;
-- name: InsertLibraryAsset :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name, width, height, size_bytes)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
-- name: GetLibraryAssets :many
SELECT * FROM assets
WHERE owner_id = ? AND type = ?
ORDER BY created_at DESC;
-- name: SearchLibraryAssets :many
SELECT * FROM assets
WHERE owner_id = sqlc.arg(owner_id) AND type = sqlc.arg(type) AND name LIKE sqlc.arg(term)
ORDER BY created_at DESC;
-- name: GetLibraryAsset :one
SELECT * FROM assets
WHERE id = ? AND owner_id = ? AND type = ?;
-- name: ReplaceLibraryAsset :exec
UPDATE assets
SET file_name = ?, width = ?, height = ?, size_bytes = ?, updated_at = NOW()
WHERE id = ? AND owner_id = ? AND type = ?;
-- name: InsertMusic :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name)
VALUES (?, ?, ?, 'music', ?, ?);
-- name: GetMusicLibrary :many
SELECT * FROM assets
WHERE owner_id = ? AND type = 'music' AND uploaded_at IS NOT NULL
ORDER BY created_at DESC;
-- name: SearchMusicLibrary :many
SELECT * FROM assets
WHERE owner_id = sqlc.arg(owner_id) AND type = 'music' AND uploaded_at IS NOT NULL
  AND name LIKE sqlc.arg(term)
ORDER BY created_at DESC;
-- name: GetMusicTrack :one
SELECT * FROM assets
WHERE id = ? AND owner_id = ? AND type = 'music';
-- name: FinishMusicUpload :execresult
UPDATE assets
SET uploaded_at = NOW(), size_bytes = ?
WHERE id = ? AND owner_id = ? AND type = 'music' AND uploaded_at IS NULL;
-- name: SumOwnedAssetBytes :one
SELECT CAST(COALESCE(SUM(size_bytes), 0) AS SIGNED) AS total_bytes FROM assets
WHERE owner_id = ?;
-- name: ListAbandonedMusicUploads :many
SELECT id, owner_id, file_path FROM assets
WHERE type = 'music' AND uploaded_at IS NULL AND created_at < ?
ORDER BY created_at
LIMIT 100;
