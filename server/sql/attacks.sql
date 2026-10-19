-- name: ListCharacterAttacks :many
SELECT * FROM attacks
WHERE character_id = ? AND owner_id = ?
ORDER BY sort_order, id;
-- name: InsertAttack :execresult
INSERT INTO attacks (id, owner_id, character_id, sort_order)
SELECT sqlc.arg(id), characters.owner_id, characters.id, COALESCE(MAX(attacks.sort_order) + 1, 0)
FROM characters
LEFT JOIN attacks ON attacks.character_id = characters.id
WHERE characters.id = sqlc.arg(character_id) AND characters.owner_id = sqlc.arg(owner_id)
GROUP BY characters.id, characters.owner_id;
-- name: ListAttackOrder :many
SELECT id FROM attacks
WHERE character_id = ? AND owner_id = ?
ORDER BY sort_order, id
FOR UPDATE;
-- name: SetAttackOrder :exec
UPDATE attacks
SET sort_order = ?
WHERE id = ? AND character_id = ? AND owner_id = ?;
-- name: GetAttack :one
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
DELETE FROM attacks
WHERE character_id = ? AND owner_id = ?;
