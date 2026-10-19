-- name: ListSpellsAtLevel :many
SELECT * FROM spells
WHERE character_id = ? AND owner_id = ? AND level = ?
ORDER BY sort_order, id;
-- name: ListPreparedSpells :many
SELECT * FROM spells
WHERE character_id = ? AND owner_id = ? AND is_prepared = TRUE
ORDER BY level, sort_order, id;
-- name: ListCharacterSpells :many
SELECT * FROM spells
WHERE character_id = ? AND owner_id = ?
ORDER BY level, sort_order, id;
-- name: CountSpellsByLevel :many
SELECT level, COUNT(*) AS total FROM spells
WHERE character_id = ? AND owner_id = ?
GROUP BY level
ORDER BY level;
-- name: InsertSpell :execresult
INSERT INTO spells (id, owner_id, character_id, level, sort_order)
SELECT sqlc.arg(id), characters.owner_id, characters.id, sqlc.arg(level), COALESCE(MAX(spells.sort_order) + 1, 0)
FROM characters
LEFT JOIN spells ON spells.character_id = characters.id AND spells.level = sqlc.arg(level)
WHERE characters.id = sqlc.arg(character_id) AND characters.owner_id = sqlc.arg(owner_id)
GROUP BY characters.id, characters.owner_id;
-- name: ListSpellOrder :many
SELECT id FROM spells
WHERE character_id = ? AND owner_id = ? AND level = ?
ORDER BY sort_order, id
FOR UPDATE;
-- name: SetSpellOrder :exec
UPDATE spells
SET sort_order = ?
WHERE id = ? AND character_id = ? AND owner_id = ? AND level = ?;
-- name: GetSpell :one
SELECT * FROM spells
WHERE id = ? AND character_id = ? AND owner_id = ? AND level = ?;
-- name: UpdateSpell :execresult
UPDATE spells
SET name = ?,
    school = ?,
    components = ?,
    casting_time = ?,
    casting_range = ?,
    duration = ?,
    description = ?,
    is_prepared = ?
WHERE id = ? AND character_id = ? AND owner_id = ? AND level = ?;
-- name: DeleteSpell :execresult
DELETE FROM spells
WHERE id = ? AND character_id = ? AND owner_id = ? AND level = ?;
-- name: ListSpellSlots :many
SELECT * FROM spell_slots
WHERE character_id = ? AND owner_id = ?
ORDER BY level;
-- name: GetSpellSlots :one
SELECT * FROM spell_slots
WHERE character_id = ? AND owner_id = ? AND level = ?;
-- name: UpsertSpellSlots :execresult
INSERT INTO spell_slots (character_id, owner_id, level, slots, used)
SELECT characters.id, characters.owner_id, sqlc.arg(level), sqlc.arg(slots), sqlc.arg(used)
FROM characters
WHERE characters.id = sqlc.arg(character_id) AND characters.owner_id = sqlc.arg(owner_id)
ON DUPLICATE KEY UPDATE slots = sqlc.arg(slots), used = sqlc.arg(used);
-- name: DeleteCharacterSpells :exec
DELETE FROM spells
WHERE character_id = ? AND owner_id = ?;
-- name: DeleteCharacterSpellSlots :exec
DELETE FROM spell_slots
WHERE character_id = ? AND owner_id = ?;
