-- name: InsertAvatar :exec
INSERT INTO assets 
(id, owner_id, file_path, type, file_name, name)
VALUES (?, ?, ?, 1, ?, ?);

-- name: GetImage :one
SELECT id, file_path, preview_path, owner_id FROM assets
WHERE id = ? AND (type = 0 OR type = 1 OR type = 2);

-- name: GetUserImage :one
SELECT id, name, file_path, preview_path, owner_id FROM assets
WHERE id = ? AND owner_id = ? AND (type = 0 OR type = 1 OR type = 2);

-- name: InsertMap :exec
INSERT INTO assets 
(id, owner_id, file_path, preview_path, type, file_name, name)
VALUES (?, ?, ?, ?, 0, ?, ?);

-- name: UpdateAssetFileName :exec
UPDATE assets 
SET file_name = ?
WHERE id = ? AND owner_id = ?;

-- name: UpdateAssetName :exec
UPDATE assets 
SET name = ?
WHERE id = ? AND owner_id = ?;

-- name: GetUserMaps :many
SELECT id, file_path, preview_path, file_name, name
FROM assets
WHERE owner_id = ? AND type = 0
ORDER BY created_at DESC;

-- name: DeleteUsersAsset :exec
DELETE FROM assets
WHERE id = ? AND owner_id = ?;
