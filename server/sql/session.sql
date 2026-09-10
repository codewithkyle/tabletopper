-- GetSession is the read on every authenticated request.
--
-- THE NAME COMES OFF THE JOIN AND THE AVATAR OFF THE ROW, which looks
-- inconsistent and is not. The name is the account's own and the settings
-- dialog can change it while the reader is sitting in the app, so a copy on
-- this row would go stale on their other devices until those sessions expired.
-- The avatar comes from Clerk, this app cannot change it, and a login is the
-- only thing that ever refreshes it -- which is what a copy on a session row is
-- for. The preference columns below are joined for the first reason.
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

-- THE THREE ROOM STATEMENTS. Membership in a room is a column on the session
-- row, not a table: a session is at one table at a time, and the column has
-- been on this row since 20260227173035 waiting for something to write it.
--
-- Two of the three key on `hash` rather than on `id`, because that is what the
-- store holds for the request in flight -- EndSession and RefreshSession key on
-- it for the same reason. The third keys on the room, because it is the sweep
-- that empties one.
--
-- THE STATEMENT THAT LISTS A ROOM'S MEMBERS IS AT THE BOTTOM OF THIS FILE, and
-- it is the Player List window's fallback rather than its source: who is at the
-- table is answered by the running room in internal/hub, and this answers the
-- moments when there is no running room to ask.

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

-- ClearUserRoomSessions is one person's half of ClearRoomSessions above: what a
-- kick owes the browser it just removed. It is scoped by both columns because a
-- kick removes one player and leaves the rest of the table where it is, and it
-- names the user rather than a hash because the person being removed may have
-- several tabs open and all of them are leaving.
-- name: ClearUserRoomSessions :execresult
UPDATE sessions
SET room_id = NULL, character_id = NULL
WHERE room_id = ? AND user_id = ?;

-- ListRoomMembers is who is in a room according to the session rows, which is
-- what the player list falls back to while the room is not live in the hub.
--
-- THAT WINDOW IS SMALL AND THE LIST IS NOT THE LIVE ONE. A room is live from
-- the moment somebody's socket opens, so this answers the page between its
-- render and its first frame, and it answers with the membership rather than
-- with who is connected -- there is nobody connected to a room that is not
-- running. It also cannot see the GM, whose membership is ownership of the
-- rooms row rather than a room_id on their session.
--
-- DISTINCT, because one person with two tabs is two rows here and one member.
-- The avatar is denormalised onto every session row from the same Clerk profile
-- and the name comes off the join, so those columns agree across a user's rows
-- and the distinct collapses them to one.
--
-- THE CHARACTER IS JOINED RATHER THAN COPIED, so a character renamed between
-- sitting down and reading the window is read under the name it has now. The
-- join is LEFT because a session whose character has since been deleted is
-- still a person at the table, and the empty name renders as their account's.
--
-- It is also the one column that can put somebody in this list twice: the
-- character is chosen per join, so two tabs can hold two characters. The live
-- list keys on the user and cannot, which is one more way this fallback is not
-- the real answer.
-- name: ListRoomMembers :many
SELECT DISTINCT s.user_id, u.username, s.profile_image_url, u.avatar_asset_id, c.name AS character_name
FROM sessions s
INNER JOIN users u ON u.id = s.user_id
LEFT JOIN characters c ON c.id = s.character_id
WHERE s.room_id = ? AND s.expires_at > NOW()
ORDER BY u.username;
