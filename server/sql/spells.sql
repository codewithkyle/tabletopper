-- Spells and their slots. Like inventory, the row is the unit of work rather
-- than the panel, and every statement is scoped by owner_id as well as by the
-- ids that arrive in the URL: a request can name any spell it likes and still
-- only reach its own.
--
-- The level is scoped too, and that is not belt-and-braces. It arrives in the
-- URL the same way the spell id does -- /characters/{id}/spells/{level}/{spellId}
-- -- and a spell cannot change level, so a statement that ignored it would let
-- .../spells/9/<a-cantrip> succeed and quietly make the URL a lie.

-- name: ListSpellsAtLevel :many
SELECT * FROM spells
WHERE character_id = ? AND owner_id = ? AND level = ?
ORDER BY id;

-- ULIDs sort lexicographically by creation time, so ORDER BY id is insertion
-- order -- the order the rows were added, which is the order they were on screen
-- when they were added. idx_spells_character_level (character_id, level, id)
-- serves this without a filesort. Alphabetical would be worse than useless: a
-- freshly added blank row would slide around the list as its name was typed.

-- name: ListPreparedSpells :many
SELECT * FROM spells
WHERE character_id = ? AND owner_id = ? AND is_prepared = TRUE
ORDER BY level, id;

-- name: CountSpellsByLevel :many
-- Feeds the Spell Slots panel on the Character tab, which says how many spells
-- each level holds beside the counters for it. Levels with no spells are absent
-- from the result; ten rows render regardless and a missing level counts zero.
SELECT level, COUNT(*) AS total FROM spells
WHERE character_id = ? AND owner_id = ?
GROUP BY level
ORDER BY level;

-- name: InsertSpell :execresult
-- FOUR COLUMNS AND NO MORE. Every other column has a schema default, so an add
-- has nowhere to put spell data even if a caller tried to supply some -- the
-- same property InsertInventoryItem has, and for the same reason: a wide insert
-- reachable from a narrow request is how defaults get written over real values.
--
-- level is the fourth because it is the only thing about a new spell that is not
-- a default: it comes from the page the button is on, and it never changes
-- afterwards. UpdateSpell does not name it.
--
-- INSERT ... SELECT rather than VALUES, so the character is the guard. A plain
-- VALUES would take character_id straight from the URL and owner_id from the
-- session, which are consistent with each other but say nothing about whether
-- that character is this user's. Selecting owner_id and id off the characters
-- row means a character that is not this user's matches nothing and inserts
-- nothing, and the handler reads that as a 404.
INSERT INTO spells (id, owner_id, character_id, level)
SELECT sqlc.arg(id), characters.owner_id, characters.id, sqlc.arg(level)
FROM characters
WHERE characters.id = sqlc.arg(character_id) AND characters.owner_id = sqlc.arg(owner_id);

-- name: GetSpell :one
-- Read back after an insert, so the markup for a new row comes from the row
-- rather than from a copy of the schema's defaults kept in Go. school starts at
-- Evocation and there is exactly one place that says so.
SELECT * FROM spells
WHERE id = ? AND character_id = ? AND owner_id = ? AND level = ?;

-- name: UpdateSpell :execresult
UPDATE spells
SET name = ?,
    school = ?,
    components = ?,
    casting_time = ?,
    casting_range = ?,
    duration = ?,
    description = ?,
    is_prepared = ?
WHERE id = ? AND character_id = ? AND owner_id = ? AND level = ?;

-- name: DeleteSpell :execresult
DELETE FROM spells
WHERE id = ? AND character_id = ? AND owner_id = ? AND level = ?;

-- name: ListSpellSlots :many
-- Every level's counters at once, for the Spell Slots panel. It is the whole
-- reason that panel exists: resetting `used` after a long rest touches nine
-- levels, and doing it a page at a time was nine page loads.
--
-- Nothing seeds this table, so a character who has never set a count has no rows
-- here at all. Ten levels render regardless and the missing ones read as the
-- zeroes they would have held.
SELECT * FROM spell_slots
WHERE character_id = ? AND owner_id = ?
ORDER BY level;

-- name: GetSpellSlots :one
-- One level's counters. Nothing seeds this table -- a row appears the first time
-- a level is given a count -- so a level nobody has touched has no row at all,
-- and the handler reads sql.ErrNoRows as the zeroes it would have held. Cantrips
-- never reach this: they have no slots, so their page asks nothing.
SELECT * FROM spell_slots
WHERE character_id = ? AND owner_id = ? AND level = ?;

-- name: UpsertSpellSlots :execresult
-- The one statement in the schema that is an upsert, because (character_id,
-- level) is the whole identity of a slot row: there is exactly one per character
-- per level and no way to create a second, so the first save of a level has
-- nothing to update and every save after it has nothing to insert.
--
-- The guard is InsertSpell's -- the SELECT is off characters, so a character
-- that is not this user's matches nothing and neither branch runs. Zero matched
-- rows means that and only that, and it means it ONLY because database.Open
-- sets ClientFoundRows. Measured against MySQL 9.6 through that connection:
-- an insert reports 1, an update reports 2, and a save that writes the values
-- already there reports 1. Without the flag that last case reports 0, which is
-- the reply a debounce lands in constantly -- the handler would answer a repeat
-- save of an unchanged row by telling the user their character was gone.
--
-- The update clause repeats the named parameters instead of reading them back
-- with VALUES(), which has been deprecated since MySQL 8.0.20. The row-alias
-- form that replaced it is not accepted after INSERT ... SELECT.
INSERT INTO spell_slots (character_id, owner_id, level, slots, used)
SELECT characters.id, characters.owner_id, sqlc.arg(level), sqlc.arg(slots), sqlc.arg(used)
FROM characters
WHERE characters.id = sqlc.arg(character_id) AND characters.owner_id = sqlc.arg(owner_id)
ON DUPLICATE KEY UPDATE slots = sqlc.arg(slots), used = sqlc.arg(used);
