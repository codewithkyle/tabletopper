-- The seven sections of a stat block, as rows. Every write below is scoped by
-- all three of id, monster_id and owner_id: the first two arrive in the URL and
-- the third comes from the session, so a request can name any row it likes and
-- still only reach its own.

-- name: ListMonsterActions :many
-- ORDER BY kind, id IS THE STAT BLOCK'S ORDER, and it is the ENUM that makes it
-- so: MySQL sorts ENUM values by their definition order, and the members were
-- declared traits first and regional effects last for exactly this. Within a
-- section, ULIDs sort lexicographically by creation time, so id is insertion
-- order -- Multiattack leads because it is written first.
--
-- idx_monster_actions_monster (monster_id, kind, id) serves the whole thing
-- without a filesort, and one read fills all seven sections.
SELECT * FROM monster_actions
WHERE monster_id = ? AND owner_id = ?
ORDER BY kind, id;

-- name: InsertMonsterAction :execresult
-- FOUR VALUES, THREE OF THEM IDS AND THE FOURTH THE SECTION. name and
-- description carry schema defaults, so an add has nowhere to put action data
-- even if a caller supplied some -- the row comes back blank and the GM types
-- into it.
--
-- INSERT ... SELECT rather than VALUES, so the monster is the guard. A plain
-- VALUES would take monster_id from the URL and owner_id from the session,
-- which agree with each other and say nothing about whether that monster
-- belongs to this user -- it would hang rows off a stranger's stat block that
-- only the sender could see. Selecting both off the monsters row means a
-- monster that is not this user's matches nothing and inserts nothing, and the
-- handler reads that as a 404.
--
-- kind arrives from the path and is matched against the Go allowlist before
-- this runs. The column is an ENUM as well, so a value that got past the
-- allowlist would be refused here too -- as a 500 rather than as a 404, which
-- is why the allowlist is the one that does the work.
INSERT INTO monster_actions (id, owner_id, monster_id, kind)
SELECT sqlc.arg(id), monsters.owner_id, monsters.id, sqlc.arg(kind)
FROM monsters
WHERE monsters.id = sqlc.arg(monster_id) AND monsters.owner_id = sqlc.arg(owner_id);

-- name: CopyMonsterAction :exec
-- One row of an imported monster's stat block. It is the only insert in this
-- file that is a plain VALUES, and the reason InsertMonsterAction above is not
-- does not apply to it: the monster it names was created by the same request a
-- moment earlier, under the session's own owner id, off a CopyMonster the share
-- row scoped. There is no id here that came from the visitor to be guarded
-- against.
--
-- kind, name and description come out of the source row rather than off a form,
-- read by ListMonsterActions and handed back column for column, so an import
-- cannot invent a section the ENUM would refuse.
--
-- THE IDS ARE MINTED IN ORDER AND THAT IS LOAD-BEARING. ListMonsterActions
-- sorts by kind then id, so id is the order the sections print in -- see
-- copyMonsterActions for why ulid.Make is not what mints them.
INSERT INTO monster_actions (id, owner_id, monster_id, kind, name, description)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetMonsterAction :one
-- Read back after an insert, so the markup for a new row comes from the row
-- rather than from a copy of the schema's defaults kept in Go.
SELECT * FROM monster_actions
WHERE id = ? AND monster_id = ? AND owner_id = ?;

-- name: UpdateMonsterAction :execresult
-- kind is not in the SET list and never will be. A row cannot change section:
-- the kind identifies it as much as its id does -- it is half the URL the row
-- posts to and half the index it is read back by -- and moving a Bite from
-- Actions to Reactions is deleting a row and adding another.
UPDATE monster_actions
SET name = ?, description = ?
WHERE id = ? AND monster_id = ? AND owner_id = ?;

-- name: DeleteMonsterAction :execresult
DELETE FROM monster_actions
WHERE id = ? AND monster_id = ? AND owner_id = ?;

-- name: DeleteMonsterActions :exec
-- The actions step of a monster delete. Nothing cascades in this schema --
-- there are no foreign keys -- so every table carrying a monster_id is named by
-- the handler, and a row left behind here is unreachable: no page can open it
-- and no later delete will ever find it.
DELETE FROM monster_actions
WHERE monster_id = ? AND owner_id = ?;
