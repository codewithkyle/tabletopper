-- Inventory is the only part of the character sheet where the row is the unit of
-- work rather than the panel. Every write below is scoped by all three of id,
-- character_id and owner_id: the first two arrive in the URL and the third comes
-- from the session, so a request can name any item it likes and still only reach
-- its own.

-- name: ListCharacterInventory :many
SELECT * FROM inventory
WHERE character_id = ? AND owner_id = ?
ORDER BY id;

-- ULIDs sort lexicographically by creation time, so ORDER BY id above and here
-- is insertion order -- the order the rows were added, which is the order they
-- were on screen when they were added. idx_inventory_character (character_id, id)
-- serves both without a filesort.

-- name: ListEquippedInventory :many
SELECT * FROM inventory
WHERE character_id = ? AND owner_id = ? AND equipped = TRUE
ORDER BY id;

-- name: InsertInventoryItem :execresult
-- THREE COLUMNS AND NO MORE. Every other column has a schema default, so an add
-- has nowhere to put item data even if a caller tried to supply some -- the same
-- property CreateCharacterFromName has, and for the same reason: a wide insert
-- reachable from a narrow request is how defaults get written over real values.
--
-- INSERT ... SELECT rather than VALUES, so the character is the guard. A plain
-- VALUES would take character_id straight from the URL and owner_id from the
-- session, which are consistent with each other but say nothing about whether
-- that character is this user's -- it would hang items off a stranger's sheet
-- that only the sender could see. Selecting owner_id and id off the characters
-- row means a character that is not this user's matches nothing and inserts
-- nothing, and the handler reads that as a 404.
INSERT INTO inventory (id, owner_id, character_id)
SELECT sqlc.arg(id), characters.owner_id, characters.id
FROM characters
WHERE characters.id = sqlc.arg(character_id) AND characters.owner_id = sqlc.arg(owner_id);

-- name: GetInventoryItem :one
-- Read back after an insert, so the markup for a new row comes from the row
-- rather than from a copy of the schema's defaults kept in Go. quantity defaults
-- to 1 and there is exactly one place that says so.
SELECT * FROM inventory
WHERE id = ? AND character_id = ? AND owner_id = ?;

-- name: UpdateInventoryItem :execresult
UPDATE inventory
SET name = ?, quantity = ?, `value` = ?, weight = ?, equipped = ?, description = ?
WHERE id = ? AND character_id = ? AND owner_id = ?;

-- name: DeleteInventoryItem :execresult
DELETE FROM inventory
WHERE id = ? AND character_id = ? AND owner_id = ?;
