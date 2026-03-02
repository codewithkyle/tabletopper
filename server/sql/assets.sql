-- name: InsertAvatar :exec
INSERT INTO assets 
(id, owner_id, file_path, type)
VALUES (?, ?, ?, 1);

-- name: GetImage :one
SELECT id, file_path, owner_id FROM assets
WHERE id = ? AND (type = 0 OR type = 1 OR type = 2);

