-- name: ListSavedRolls :many
SELECT id, label, expr FROM saved_rolls
WHERE owner_id = ?
ORDER BY id;
-- name: InsertSavedRoll :exec
INSERT INTO saved_rolls (id, owner_id, label, expr)
VALUES (?, ?, ?, ?);
-- name: DeleteSavedRoll :execresult
DELETE FROM saved_rolls
WHERE id = ? AND owner_id = ?;
