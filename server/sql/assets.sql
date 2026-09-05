-- Every query here is scoped to the owner except GetImage, which is the one
-- read that deliberately is not: a map or token is shown to every player at
-- the table, so any signed-in user may fetch any image by id. Ownership
-- gates the writes.

-- name: InsertAvatar :exec
INSERT INTO assets
(id, owner_id, file_path, type, file_name, name)
VALUES (?, ?, ?, 'avatar', ?, ?);

-- name: InsertMap :exec
INSERT INTO assets
(id, owner_id, file_path, preview_path, type, file_name, name)
VALUES (?, ?, ?, ?, 'map', ?, ?);

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
