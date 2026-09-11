-- name: ListCharacterInventory :many
SELECT * FROM inventory
WHERE character_id = ? AND owner_id = ?
ORDER BY id;
-- name: ListEquippedInventory :many
SELECT * FROM inventory
WHERE character_id = ? AND owner_id = ? AND equipped = TRUE
ORDER BY id;
-- name: InsertInventoryItem :execresult
INSERT INTO inventory (id, owner_id, character_id)
SELECT sqlc.arg(id), characters.owner_id, characters.id
FROM characters
WHERE characters.id = sqlc.arg(character_id) AND characters.owner_id = sqlc.arg(owner_id);
-- name: GetInventoryItem :one
SELECT * FROM inventory
WHERE id = ? AND character_id = ? AND owner_id = ?;
-- name: UpdateInventoryItem :execresult
UPDATE inventory
SET name = ?, quantity = ?, `value` = ?, weight = ?, equipped = ?, description = ?
WHERE id = ? AND character_id = ? AND owner_id = ?;
-- name: DeleteInventoryItem :execresult
DELETE FROM inventory
WHERE id = ? AND character_id = ? AND owner_id = ?;
-- name: DeleteCharacterInventory :exec
DELETE FROM inventory
WHERE character_id = ? AND owner_id = ?;
