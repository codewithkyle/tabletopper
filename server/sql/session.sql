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

-- THE FOUR ROOM STATEMENTS. Membership in a room is a column on the session
-- row, not a table: a session is at one table at a time, and the column has
-- been on this row since 20260227173035 waiting for something to write it.
--
-- Three of the four key on `hash` rather than on `id`, because that is what the
-- store holds for the request in flight -- EndSession and RefreshSession key on
-- it for the same reason. The fourth keys on the room, because it is the sweep
-- that empties one.

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

-- The members panel on the room page, as the initial render draws it.
--
-- IT READS SESSIONS AND NOT USERS, because username and profile_image_url are
-- already denormalised onto the session row -- see internal/session for why
-- they are copied there and the preferences are not. Joining users to fetch two
-- columns this row already carries would be a join for nothing.
--
-- DISTINCT COLLAPSES ONE PERSON WITH TWO BROWSERS INTO ONE MEMBER. It is the
-- user at the table who is a member, not the tab. Two sessions of one user that
-- picked different characters are two rows even so, which is the honest answer:
-- the panel would be lying if it hid one of them.
--
-- The expiry check is here rather than left to the sweep, because a session
-- that ran out an hour ago is somebody who left rather than somebody sitting
-- quietly, and the hourly sweep has not come round yet.
--
-- THE JOIN TO characters IS WHAT MAKES DISTINCT HONEST. The panel names who is
-- at the table and what they brought, so the character has to be a name rather
-- than an id -- and without it two sessions of one person who picked different
-- characters would collapse into two rows that read identically. It is a LEFT
-- JOIN on a primary key, so it costs one lookup per member and it is NULL for
-- the two cases that are not an error: somebody who joined with no character,
-- and somebody whose character was deleted afterwards.
-- name: ListRoomMembers :many
SELECT DISTINCT s.user_id, s.username, s.profile_image_url, s.character_id, c.name AS character_name
FROM sessions s
LEFT JOIN characters c ON c.id = s.character_id
WHERE s.room_id = ? AND s.expires_at > NOW()
ORDER BY s.username;
