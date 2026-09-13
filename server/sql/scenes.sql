-- name: CreateScene :exec
INSERT INTO scenes (id, owner_id, name, body, preview_id)
VALUES (?, ?, ?, ?, ?);
-- name: CountScenes :one
SELECT COUNT(*) FROM scenes
WHERE owner_id = ?;
-- name: ListScenes :many
SELECT s.id, s.name, s.autosave, s.created_at, s.updated_at, a.id AS preview_id
FROM scenes s
LEFT JOIN assets a ON a.id = s.preview_id AND a.owner_id = s.owner_id AND a.type = 'map'
WHERE s.owner_id = ?
ORDER BY s.name;
-- name: GetScene :one
SELECT id, owner_id, name, body, autosave, preview_id, created_at, updated_at
FROM scenes
WHERE id = ? AND owner_id = ?;
-- name: UpdateSceneBody :execresult
UPDATE scenes
SET body = ?, preview_id = ?
WHERE id = ? AND owner_id = ?;
-- name: RenameScene :execresult
UPDATE scenes
SET name = ?
WHERE id = ? AND owner_id = ?;
-- name: SetSceneAutosave :execresult
UPDATE scenes
SET autosave = ?
WHERE id = ? AND owner_id = ?;
-- name: DeleteScene :execresult
DELETE FROM scenes
WHERE id = ? AND owner_id = ?;
-- name: DuplicateScene :execresult
INSERT INTO scenes (id, owner_id, name, body, autosave, preview_id)
SELECT sqlc.arg(new_id), original.owner_id, sqlc.arg(name), original.body, original.autosave, original.preview_id
FROM scenes original
WHERE original.id = sqlc.arg(id) AND original.owner_id = sqlc.arg(owner_id);
-- name: ForgetScene :execresult
UPDATE rooms
SET scene_id = NULL
WHERE scene_id = ? AND owner_id = ?;
