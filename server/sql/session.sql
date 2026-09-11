-- name: GetSession :one
SELECT s.id, s.profile_image_url, s.user_id, s.character_id, s.room_id,
       s.created_at, s.refreshed_at,
       u.username, u.avatar_asset_id,
       u.theme, u.timezone, u.date_format, u.time_format,
       u.follow_turn, u.show_blood, u.ping_volume, u.onboarded_at
FROM sessions s
INNER JOIN users u ON u.id = s.user_id
WHERE s.expires_at > NOW() AND s.hash = ?;
-- name: EndSession :exec
UPDATE sessions
SET expires_at = NOW()
WHERE hash = ?;
-- name: StartSession :exec
INSERT INTO sessions
(id, hash, profile_image_url, user_id, expires_at)
VALUES (?, ?, ?, ?, ?);
-- name: RefreshSession :execresult
UPDATE sessions
SET refreshed_at = NOW(), expires_at = sqlc.arg(expires_at)
WHERE hash = sqlc.arg(hash)
  AND expires_at > NOW()
  AND refreshed_at < sqlc.arg(refresh_cutoff)
  AND created_at > sqlc.arg(lifetime_cutoff);
-- name: DeleteExpiredSessions :execresult
DELETE FROM sessions
WHERE expires_at < sqlc.arg(cutoff);
-- name: SetSessionRoom :execresult
UPDATE sessions
SET room_id = ?, character_id = ?
WHERE hash = ?;
-- name: ClearSessionRoom :execresult
UPDATE sessions
SET room_id = NULL, character_id = NULL
WHERE hash = ?;
-- name: ClearRoomSessions :execresult
UPDATE sessions
SET room_id = NULL, character_id = NULL
WHERE room_id = ?;
-- name: ClearUserRoomSessions :execresult
UPDATE sessions
SET room_id = NULL, character_id = NULL
WHERE room_id = ? AND user_id = ?;
-- name: ListRoomMembers :many
SELECT DISTINCT s.user_id, u.username, s.profile_image_url, u.avatar_asset_id, c.name AS character_name
FROM sessions s
INNER JOIN users u ON u.id = s.user_id
LEFT JOIN characters c ON c.id = s.character_id
WHERE s.room_id = ? AND s.expires_at > NOW()
ORDER BY u.username;
