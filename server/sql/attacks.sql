-- Attacks are the second part of the sheet where the row is the unit of work
-- rather than the panel. Every write below is scoped by all three of id,
-- character_id and owner_id: the first two arrive in the URL and the third comes
-- from the session, so a request can name any attack it likes and still only
-- reach its own.

-- name: ListCharacterAttacks :many
-- ULIDs sort lexicographically by creation time, so ORDER BY id is insertion
-- order -- the order the rows were added, which is the order they were on screen
-- when they were added. idx_attacks_character (character_id, id) serves it
-- without a filesort.
SELECT * FROM attacks
WHERE character_id = ? AND owner_id = ?
ORDER BY id;

-- name: InsertAttack :execresult
-- THREE COLUMNS AND NO MORE, the shape InsertInventoryItem has. Every other
-- column carries a schema default, so an add has nowhere to put attack data even
-- if a caller supplied some.
--
-- INSERT ... SELECT rather than VALUES, so the character is the guard. A plain
-- VALUES would take character_id from the URL and owner_id from the session,
-- which are consistent with each other but say nothing about whether that
-- character belongs to this user -- it would hang attacks off a stranger's sheet
-- that only the sender could see. Selecting both off the characters row means a
-- character that is not this user's matches nothing and inserts nothing, and the
-- handler reads that as a 404.
INSERT INTO attacks (id, owner_id, character_id)
SELECT sqlc.arg(id), characters.owner_id, characters.id
FROM characters
WHERE characters.id = sqlc.arg(character_id) AND characters.owner_id = sqlc.arg(owner_id);

-- name: GetAttack :one
-- Read back after an insert, so the markup for a new row comes from the row
-- rather than from a copy of the schema's defaults kept in Go.
SELECT * FROM attacks
WHERE id = ? AND character_id = ? AND owner_id = ?;

-- name: UpdateAttack :execresult
UPDATE attacks
SET name = ?, attack_bonus = ?, damage = ?, damage_type = ?, mastery = ?, notes = ?
WHERE id = ? AND character_id = ? AND owner_id = ?;

-- name: DeleteAttack :execresult
DELETE FROM attacks
WHERE id = ? AND character_id = ? AND owner_id = ?;

-- name: DeleteCharacterAttacks :exec
-- The attacks step of a character delete. Nothing cascades in this schema --
-- there are no foreign keys -- so every table carrying a character_id has to be
-- named by the handler, and a row left behind here is unreachable: no page can
-- open it and no later delete will ever find it.
DELETE FROM attacks
WHERE character_id = ? AND owner_id = ?;
