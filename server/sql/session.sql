-- name: GetSession :one
SELECT id, username, profile_image_url, user_id, character_id, room_id
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
