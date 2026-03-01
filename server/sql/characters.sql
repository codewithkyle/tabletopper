-- name: GetCharacters :many
SELECT id, name, level, race, classes, ac, current_hp, asset_id FROM characters
WHERE owner_id = ?
ORDER BY created_at DESC;
