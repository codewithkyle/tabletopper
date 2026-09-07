-- migrate:up
-- A ROOM IS A TABLE A GM KEEPS, NOT A SESSION THEY START. The row exists until
-- its owner deletes it, and it is either open -- it has a four-character code
-- and players may join -- or closed, which is the same room with nobody in it
-- and no way in. Reopening mints a new code. The alternative was an ephemeral
-- room created per game night and thrown away afterwards, which is this design
-- with a delete on the end; what makes the row worth keeping is the snapshot
-- three columns down, since a room that remembers where the pawns were is a
-- room worth coming back to next Saturday.
--
-- DROP AND CREATE RATHER THAN ALTER, AND THAT IS SAFE HERE BECAUSE NOTHING HAS
-- EVER WRITTEN THIS TABLE. It was created by 20260227175838 alongside the rest
-- of the original schema and no query in server/sql has ever named it -- the
-- VTT it was for is only being built now. So there are no rows to preserve and
-- no reason to spend four ALTERs and a backfill preserving them. Anything that
-- did somehow land in it was a room nobody could reach.
--
-- THE UNIQUE KEY IS THE REASON THE OLD SHAPE COULD NOT BE KEPT. It was
-- UNIQUE (code, is_open), which permits exactly one open and one CLOSED room
-- per code. Codes are recycled -- there are about 920,000 of them and a table
-- runs for years -- so the second room to close on a code it once held would
-- collide with the first and the close would fail, in the middle of a session,
-- with an error about a duplicate key on a column the GM never saw.
--
-- code IS NULLABLE AND THAT IS WHAT REPLACES THE COMPOSITE KEY. A closed room
-- gives its code back, so the column is NULL and UNIQUE (code) permits any
-- number of closed rooms -- a MySQL unique index does not consider two NULLs
-- equal. One open room per code is then the database's rule rather than the
-- handler's, and it is the rule the join actually depends on, since the join
-- looks a room up by code and must never find two.
--
-- THERE IS NO is_open. Open is `closed_at IS NULL`, which is one column that
-- cannot disagree with itself. A flag beside a timestamp is two facts about
-- one event, and 20260906190000 already refused that pairing for the same
-- reason: the day they disagree, the code reading one of them is right and the
-- code reading the other is also right.
--
-- THE SNAPSHOT IS THREE COLUMNS ON THIS ROW AND NOT A TABLE OF ITS OWN. Room
-- state lives in memory in one process, and what is stored here is one
-- debounced JSON copy of it so that a deploy in the middle of Saturday's game
-- is a reconnect rather than a lost session. There is exactly one per room, it
-- is overwritten in place, and no history is kept -- so a table would be a
-- table with a unique key on room_id and a join on every read. Nothing reads
-- these columns yet; the transport phase writes them. Every statement against
-- this table names its columns, so no existing reader will start dragging the
-- blob across the wire by accident when it lands.
--
-- snapshot IS NOT NULL DEFAULT (json_object()), meaning an empty object stands
-- for "no snapshot yet". sqlc maps a nullable JSON column to sql.NullString,
-- which every reader would then have to unwrap before parsing, and every other
-- JSON column in this schema is NOT NULL for exactly that reason.
--
-- snapshot_seq IS BIGINT UNSIGNED because it is the room's monotonic event
-- sequence and a client detects a gap in it to know when to resync. A 32-bit
-- counter is millions of events, which one long campaign could reach, and a
-- column that has to be widened later is a column that gets widened during an
-- incident.
--
-- idx_sessions_room_id EXISTS FOR THE TWO READS THAT FILTER SESSIONS BY ROOM:
-- the members panel, which lists who is at the table, and the close, which
-- clears room_id from every session in the room it just shut. Both would be a
-- full scan of a table that holds every login in the app.
--
-- NO COLUMN HERE IS CALLED status, list, table, tab, stack OR swap. sqlc turns
-- column names into Go identifiers, those identifiers are read in .templ
-- files, and Tailwind extracts a class-name candidate from every word in a
-- .templ file -- so a column called `status` would emit DaisyUI's whole status
-- family into the stylesheet with nothing failing anywhere. `is_locked` and
-- `closed_at` say what they mean and collide with nothing.
--
-- No FOREIGN KEY, like every other table in this schema. Nothing cascades
-- here; the deletes are written out where they happen.
DROP TABLE IF EXISTS rooms;

CREATE TABLE rooms (
    id           VARBINARY(16) NOT NULL,
    owner_id     VARBINARY(16) NOT NULL,
    name         VARCHAR(128) NOT NULL,
    code         CHAR(4) CHARACTER SET ascii NULL,
    is_locked    TINYINT(1) NOT NULL DEFAULT 0,
    snapshot     JSON NOT NULL DEFAULT (json_object()),
    snapshot_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,
    snapshot_at  DATETIME NULL,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    closed_at    DATETIME NULL,
    PRIMARY KEY (id),
    UNIQUE KEY ux_rooms_code (code),
    KEY idx_rooms_owner (owner_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE sessions
    ADD INDEX idx_sessions_room_id (room_id);

-- migrate:down
ALTER TABLE sessions
    DROP INDEX idx_sessions_room_id;

DROP TABLE IF EXISTS rooms;

CREATE TABLE IF NOT EXISTS rooms (
    id  VARBINARY(16) PRIMARY KEY NOT NULL,
    owner_id VARBINARY(16) NOT NULL,

    code CHAR(4) CHARACTER SET ascii NOT NULL,
    is_open TINYINT(1) NOT NULL DEFAULT 1,
    is_locked TINYINT(1) NOT NULL DEFAULT 0,

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    closed_at DATETIME NULL,

    UNIQUE KEY ux_rooms_code_open (code, is_open)
);
