-- name: GetSession :one
SELECT s.id, s.username, s.profile_image_url, s.user_id, s.character_id, s.room_id,
       s.created_at, s.refreshed_at,
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

-- THE THREE ROOM STATEMENTS. Membership in a room is a column on the session
-- row, not a table: a session is at one table at a time, and the column has
-- been on this row since 20260227173035 waiting for something to write it.
--
-- Two of the three key on `hash` rather than on `id`, because that is what the
-- store holds for the request in flight -- EndSession and RefreshSession key on
-- it for the same reason. The third keys on the room, because it is the sweep
-- that empties one.
--
-- THERE IS NO STATEMENT THAT LISTS A ROOM'S MEMBERS. The room page has no panel
-- to draw one in: the Player List is a window behind the Room menu that is not
-- built. It will read this table when it is, because username and
-- profile_image_url are already denormalised onto the session row.

-- name: SetSessionRoom :execresult
UPDATE sessions
SET room_id = ?, character_id = ?
WHERE hash = ?;

-- name: ClearSessionRoom :execresult
UPDATE sessions
SET room_id = NULL, character_id = NULL
WHERE hash = ?;

-- What closing or deleting a room owes every browser that was in it. The
-- character goes with the room: it was chosen for that table and means nothing
-- at the next one.
-- name: ClearRoomSessions :execresult
UPDATE sessions
SET room_id = NULL, character_id = NULL
WHERE room_id = ?;
