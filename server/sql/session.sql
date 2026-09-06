-- name: GetSession :one
SELECT s.id, s.username, s.profile_image_url, s.user_id, s.character_id, s.room_id, s.created_at,
       u.theme, u.timezone, u.date_format, u.time_format, u.onboarded_at
FROM sessions s
INNER JOIN users u ON u.id = s.user_id
WHERE s.expires_at > NOW() AND s.hash = ?;

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
