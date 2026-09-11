-- name: CreateRoom :exec
INSERT INTO rooms (id, owner_id, name, code)
VALUES (?, ?, ?, ?);
-- name: ListRooms :many
SELECT id, name, code, is_locked, created_at, closed_at
FROM rooms
WHERE owner_id = ?
ORDER BY created_at DESC;
-- name: GetRoom :one
SELECT id, owner_id, name, code, is_locked, created_at, closed_at
FROM rooms
WHERE id = ?;
-- name: GetOpenRoomByCode :one
SELECT id, name, is_locked
FROM rooms
WHERE code = ?;
-- name: SetRoomLocked :execresult
UPDATE rooms
SET is_locked = ?
WHERE id = ? AND owner_id = ? AND closed_at IS NULL;
-- name: CloseRoom :execresult
UPDATE rooms
SET code = NULL, closed_at = NOW()
WHERE id = ? AND owner_id = ? AND closed_at IS NULL;
-- name: OpenRoom :execresult
UPDATE rooms
SET code = ?, closed_at = NULL, is_locked = 0
WHERE id = ? AND owner_id = ? AND closed_at IS NOT NULL;
-- name: RenameRoom :execresult
UPDATE rooms
SET name = ?
WHERE id = ? AND owner_id = ?;
-- name: DeleteRoom :execresult
DELETE FROM rooms
WHERE id = ? AND owner_id = ?;
-- name: GetRoomSnapshot :one
SELECT name, is_locked, snapshot
FROM rooms
WHERE id = ? AND closed_at IS NULL;
-- name: SaveRoomSnapshot :execresult
UPDATE rooms
SET snapshot = ?, snapshot_seq = ?, snapshot_at = NOW()
WHERE id = ?;
-- name: KeepFailedSnapshot :exec
UPDATE rooms
SET snapshot_failed = ?, snapshot_failed_at = NOW()
WHERE id = ?;
