-- name: GetSession :one
SELECT id, username, profile_image_url, user_id, character_id, room_id, created_at
FROM sessions
WHERE expires_at > NOW() AND hash = ?;

-- name: EndSession :exec
UPDATE sessions
SET expires_at = NOW()
WHERE hash = ?;

-- name: StartSession :exec
INSERT INTO sessions
(id, hash, username, profile_image_url, user_id, expires_at)
VALUES (?, ?, ?, ?, ?, ?);

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
