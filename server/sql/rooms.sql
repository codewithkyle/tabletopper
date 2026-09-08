-- A room belongs to a GM, so every statement that changes one carries
-- `AND owner_id = ?`. Zero matched rows means not yours or not there, and both
-- answer 404 -- the handler never has to check ownership after the fact. The
-- reads are the two exceptions and each says why below.
--
-- ClientFoundRows is on in the DSN, so RowsAffected reports MATCHED rows rather
-- than changed ones. That is what lets :execresult tell "no such room" apart
-- from "already in that state": locking a room that is already locked matches
-- its row and reports 1.
--
-- The three snapshot columns are read and written by exactly two statements,
-- at the bottom of this file, and by nothing else. Every statement above lists
-- its columns so that none of them drags the blob back with it -- it is tens of
-- kilobytes and every other read here is a page or a card with no use for it.

-- name: CreateRoom :exec
INSERT INTO rooms (id, owner_id, name, code)
VALUES (?, ?, ?, ?);

-- name: ListRooms :many
SELECT id, name, code, is_locked, created_at, closed_at
FROM rooms
WHERE owner_id = ?
ORDER BY created_at DESC;

-- GetRoom is NOT owner-scoped, and it is the one statement here that is not.
-- Members read the room too, and a member is not the owner -- membership is
-- their session's room_id. So this hands back owner_id and the handler decides
-- the role from it: owner is the GM, a session pointed at this room is a
-- player, and anybody else is turned away. Nothing gated is selected; the code
-- is the only sensitive column and a member is entitled to it.
-- name: GetRoom :one
SELECT id, owner_id, name, code, is_locked, created_at, closed_at
FROM rooms
WHERE id = ?;

-- GetOpenRoomByCode is what a join looks a code up with, and it cannot match a
-- closed room: closing sets code to NULL and NULL matches nothing. So "open" is
-- enforced by the column rather than by a predicate that could be dropped.
-- It selects the id and the lock flag and nothing else, the way GetShareByToken
-- selects only what the gate needs.
-- name: GetOpenRoomByCode :one
SELECT id, name, is_locked
FROM rooms
WHERE code = ?;

-- name: SetRoomLocked :execresult
UPDATE rooms
SET is_locked = ?
WHERE id = ? AND owner_id = ? AND closed_at IS NULL;

-- Closing gives the code back, which is what lets another room take it later,
-- and `closed_at IS NULL` in the WHERE makes the statement idempotent: closing
-- a closed room matches nothing rather than moving its timestamp.
-- name: CloseRoom :execresult
UPDATE rooms
SET code = NULL, closed_at = NOW()
WHERE id = ? AND owner_id = ? AND closed_at IS NULL;

-- Reopening mints a new code rather than restoring the old one -- the old one
-- may belong to somebody else's room by now -- and clears the lock, because a
-- room nobody could join cannot meaningfully still be locked against them.
-- `closed_at IS NOT NULL` is the mirror of the close's guard.
-- name: OpenRoom :execresult
UPDATE rooms
SET code = ?, closed_at = NULL, is_locked = 0
WHERE id = ? AND owner_id = ? AND closed_at IS NOT NULL;

-- name: RenameRoom :execresult
UPDATE rooms
SET name = ?
WHERE id = ? AND owner_id = ?;

-- name: DeleteRoom :execresult
DELETE FROM rooms
WHERE id = ? AND owner_id = ?;

-- THE TWO SNAPSHOT STATEMENTS. The room's state lives in one Go process and
-- nowhere else, so a deploy in the middle of Saturday's game would lose the
-- table; these are what turn that into a reconnect. The hub writes a debounced
-- JSON blob a few seconds after the last change and on shutdown, and reads it
-- back on the first join after a restart.
--
-- The blob is not selected by any other statement in this file, and that is
-- deliberate: it is tens of kilobytes and every other read here is a page or a
-- card that has no use for it.

-- GetRoomSnapshot is the room's whole identity for the hub: the two columns the
-- row is the writer of record for, and the state to rehydrate from. It cannot
-- match a closed room -- a closed room has no session to restore, and the join
-- that would have loaded it was refused before this ran.
-- name: GetRoomSnapshot :one
SELECT name, is_locked, snapshot
FROM rooms
WHERE id = ? AND closed_at IS NULL;

-- SaveRoomSnapshot is NOT owner-scoped and cannot be: it is written by the room
-- goroutine, which has no request and no caller, on behalf of whoever is at the
-- table. The id it names came from a page load that was already checked.
--
-- It does not guard on closed_at. Closing saves one last time so that reopening
-- comes back to the pawns where they were left, which is what makes a room
-- worth keeping between Saturdays.
-- name: SaveRoomSnapshot :execresult
UPDATE rooms
SET snapshot = ?, snapshot_seq = ?, snapshot_at = NOW()
WHERE id = ?;
