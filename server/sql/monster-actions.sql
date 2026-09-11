-- name: ListMonsterActions :many
SELECT * FROM monster_actions
WHERE monster_id = ? AND owner_id = ?
ORDER BY kind, id;

-- name: InsertMonsterAction :execresult
INSERT INTO monster_actions (id, owner_id, monster_id, kind)
SELECT sqlc.arg(id), monsters.owner_id, monsters.id, sqlc.arg(kind)
FROM monsters
WHERE monsters.id = sqlc.arg(monster_id) AND monsters.owner_id = sqlc.arg(owner_id);

-- name: CopyMonsterAction :exec
INSERT INTO monster_actions (id, owner_id, monster_id, kind, name, description)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetMonsterAction :one
SELECT * FROM monster_actions
WHERE id = ? AND monster_id = ? AND owner_id = ?;

-- name: UpdateMonsterAction :execresult
UPDATE monster_actions
SET name = ?, description = ?
WHERE id = ? AND monster_id = ? AND owner_id = ?;

-- name: DeleteMonsterAction :execresult
DELETE FROM monster_actions
WHERE id = ? AND monster_id = ? AND owner_id = ?;

-- name: DeleteMonsterActions :exec
DELETE FROM monster_actions
WHERE monster_id = ? AND owner_id = ?;
