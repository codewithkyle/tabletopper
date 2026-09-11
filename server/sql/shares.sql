-- name: GetJournalShare :one
SELECT * FROM shares
WHERE resource_type = 'journal' AND resource_id = sqlc.arg(entry_id)
    AND character_id = sqlc.arg(character_id) AND owner_id = sqlc.arg(owner_id);

-- name: InsertJournalShare :execresult
INSERT INTO shares (id, owner_id, character_id, resource_type, resource_id, token, password_hash, expires_at)
SELECT sqlc.arg(id), journals.owner_id, journals.character_id, 'journal', journals.id,
    sqlc.arg(token), sqlc.arg(password_hash), sqlc.arg(expires_at)
FROM journals
WHERE journals.id = sqlc.arg(entry_id) AND journals.character_id = sqlc.arg(character_id)
    AND journals.owner_id = sqlc.arg(owner_id);

-- name: DeleteJournalShare :execresult
DELETE FROM shares
WHERE resource_type = 'journal' AND resource_id = sqlc.arg(entry_id)
    AND character_id = sqlc.arg(character_id) AND owner_id = sqlc.arg(owner_id);

-- name: GetCharacterShare :one
SELECT * FROM shares
WHERE resource_type = 'character' AND resource_id = sqlc.arg(character_id)
    AND owner_id = sqlc.arg(owner_id);

-- name: InsertCharacterShare :execresult
INSERT INTO shares (id, owner_id, character_id, resource_type, resource_id, token, password_hash, expires_at)
SELECT sqlc.arg(id), characters.owner_id, characters.id, 'character', characters.id,
    sqlc.arg(token), sqlc.arg(password_hash), sqlc.arg(expires_at)
FROM characters
WHERE characters.id = sqlc.arg(character_id) AND characters.owner_id = sqlc.arg(owner_id);

-- name: DeleteCharacterShare :execresult
DELETE FROM shares
WHERE resource_type = 'character' AND resource_id = sqlc.arg(character_id)
    AND owner_id = sqlc.arg(owner_id);

-- name: DeleteSharesForCharacter :exec
DELETE FROM shares
WHERE character_id = ? AND owner_id = ?;

-- name: GetMonsterShare :one
SELECT * FROM shares
WHERE resource_type = 'monster' AND resource_id = sqlc.arg(monster_id)
    AND owner_id = sqlc.arg(owner_id);

-- name: InsertMonsterShare :execresult
INSERT INTO shares (id, owner_id, resource_type, resource_id, token, password_hash, expires_at)
SELECT sqlc.arg(id), monsters.owner_id, 'monster', monsters.id,
    sqlc.arg(token), sqlc.arg(password_hash), sqlc.arg(expires_at)
FROM monsters
WHERE monsters.id = sqlc.arg(monster_id) AND monsters.owner_id = sqlc.arg(owner_id);

-- name: DeleteMonsterShare :execresult
DELETE FROM shares
WHERE resource_type = 'monster' AND resource_id = sqlc.arg(monster_id)
    AND owner_id = sqlc.arg(owner_id);

-- name: GetShareByToken :one
SELECT id, owner_id, character_id, resource_type, resource_id, password_hash FROM shares
WHERE token = sqlc.arg(token)
    AND (expires_at IS NULL OR expires_at > NOW());

-- name: GetSharedJournalEntry :one
SELECT journals.title, journals.body,
    characters.name, characters.level, characters.classes, characters.race, characters.asset_id
FROM journals
JOIN characters ON characters.id = journals.character_id AND characters.owner_id = journals.owner_id
WHERE journals.id = sqlc.arg(entry_id) AND journals.character_id = sqlc.arg(character_id)
    AND journals.owner_id = sqlc.arg(owner_id);

-- name: DeleteExpiredShares :execresult
DELETE FROM shares
WHERE expires_at IS NOT NULL AND expires_at < sqlc.arg(cutoff);

-- name: GetSharedCharacterAvatar :one
SELECT assets.file_path, assets.updated_at
FROM characters
JOIN assets ON assets.id = characters.asset_id
WHERE characters.id = sqlc.arg(character_id) AND characters.owner_id = sqlc.arg(owner_id);

-- name: GetSharedMonsterImage :one
SELECT assets.file_path, assets.file_name, assets.updated_at
FROM monsters
JOIN assets ON assets.id = monsters.asset_id
WHERE monsters.id = sqlc.arg(monster_id) AND monsters.owner_id = sqlc.arg(owner_id);
