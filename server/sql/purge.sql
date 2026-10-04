-- name: ListDeletedUsers :many
SELECT id FROM users
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at
LIMIT 20;
-- name: ListOwnedRoomIDs :many
SELECT id FROM rooms
WHERE owner_id = ?;
-- name: ClearSessionsInOwnedRooms :execresult
UPDATE sessions
SET room_id = NULL, character_id = NULL
WHERE room_id IN (SELECT id FROM rooms WHERE owner_id = ?);
-- name: DeleteOwnedRooms :execresult
DELETE FROM rooms
WHERE owner_id = ?;
-- name: ListOwnedCharacterIDs :many
SELECT id FROM characters
WHERE owner_id = ?;
-- name: DeleteOwnedSpells :execresult
DELETE FROM spells
WHERE owner_id = ?;
-- name: DeleteOwnedSpellSlots :execresult
DELETE FROM spell_slots
WHERE owner_id = ?;
-- name: DeleteOwnedInventory :execresult
DELETE FROM inventory
WHERE owner_id = ?;
-- name: DeleteOwnedAttacks :execresult
DELETE FROM attacks
WHERE owner_id = ?;
-- name: DeleteOwnedJournals :execresult
DELETE FROM journals
WHERE owner_id = ?;
-- name: DeleteOwnedShares :execresult
DELETE FROM shares
WHERE owner_id = ?;
-- name: DeleteOwnedCharacters :execresult
DELETE FROM characters
WHERE owner_id = ?;
-- name: DeleteOwnedMonsterActions :execresult
DELETE FROM monster_actions
WHERE owner_id = ?;
-- name: DeleteOwnedMonsters :execresult
DELETE FROM monsters
WHERE owner_id = ?;
-- name: DeleteOwnedScenes :execresult
DELETE FROM scenes
WHERE owner_id = ?;
-- name: ListOwnedAssetIDs :many
SELECT id FROM assets
WHERE owner_id = ?;
-- name: DeleteOwnedAssets :execresult
DELETE FROM assets
WHERE owner_id = ?;
-- name: DeleteUserSessions :execresult
DELETE FROM sessions
WHERE user_id = ?;
-- name: DeleteUser :execresult
DELETE FROM users
WHERE id = ? AND deleted_at IS NOT NULL;
