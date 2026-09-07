-- Every query here is scoped to the owner, the way the character statements
-- are. A monster id from someone else's manual matches nothing, so a handler
-- never checks ownership after the fact: no row means not yours or not there,
-- and both answer 404.

-- name: ListMonsters :many
-- The manual page. ORDER BY name because a bestiary is alphabetical -- the
-- roster sorts by creation date, which is right for a handful of characters and
-- wrong for a list somebody looks a monster up in. idx_monsters_owner_name is
-- exactly this read.
SELECT * FROM monsters
WHERE owner_id = ?
ORDER BY name;

-- name: SearchMonsters :many
-- The search box above the grid. Same order and same columns as the list above,
-- so a filtered manual and an unfiltered one are the same page.
--
-- THE TERM IS A PATTERN, NOT A WORD, and the caller escapes it: `%` and `_` are
-- wildcards to LIKE and ordinary characters to a person typing, so an unescaped
-- `%` here matches the whole manual. The journal search shares the helper that
-- does it.
--
-- Only the name is searched. A monster's prose lives in its description and its
-- action rows, and a manual is looked up by name -- "goblin" -- rather than
-- read through. The rows are the owner's own and there are tens of them, so the
-- LIKE scans behind idx_monsters_owner_name and reads nothing else.
--
-- The table is utf8mb4_0900_ai_ci, so this is already case- and
-- accent-insensitive.
SELECT * FROM monsters
WHERE owner_id = sqlc.arg(owner_id) AND name LIKE sqlc.arg(term)
ORDER BY name;

-- name: GetMonster :one
SELECT * FROM monsters
WHERE id = ? AND owner_id = ?;

-- name: GetMonsterAsset :one
-- What a delete needs: the name for the toast, the asset row to remove and the
-- key to delete out of the bucket. LEFT JOIN because the image is optional --
-- a monster created a minute ago has none, and the delete still has to work.
SELECT m.name, m.asset_id, a.file_path FROM monsters m
LEFT JOIN assets a ON a.id = m.asset_id
WHERE m.id = ? AND m.owner_id = ?;

-- name: CreateMonsterFromName :exec
-- THREE COLUMNS AND NOTHING ELSE, and here that is the schema's doing rather
-- than the statement's restraint: every other column in the table carries a
-- DEFAULT, so there is not one literal to write. CreateCharacterFromName still
-- names twelve, because the characters table predates the rule.
--
-- The dialog asks for a name. The stat block is filled in afterwards by the
-- editor, a panel at a time, and it autosaves -- so creation has nowhere to put
-- monster data even if a caller supplied some, which is what the params-shape
-- test refuses to let drift.
INSERT INTO monsters (id, owner_id, name)
VALUES (?, ?, ?);

-- name: CopyMonster :execresult
-- THE IMPORT, which is the one statement in this file that reads one account's
-- monster and writes another's. It is what the button on a shared monster's
-- page runs, and the copy it makes is the importer's own row from that moment
-- on -- editing either afterwards leaves the other alone.
--
-- IT IS AN INSERT ... SELECT, and here that is doing more work than it does
-- elsewhere in this schema. The source is named by the SHARE ROW -- the
-- monster and the owner both come off it, never off the request -- so a
-- visitor cannot point this at a monster the link does not name; and every
-- value written comes off the source row rather than out of a form, so an
-- import cannot smuggle a stat block in.
--
-- EVERY EDITABLE COLUMN IS NAMED, and a column left out would not fail: it
-- would silently take the schema's default, so an imported dragon would arrive
-- with AC 10 and no explanation. TestAnImportedMonsterCarriesEveryEditableColumn
-- reads the schema and holds that line.
--
-- THE SOURCE IS ALIASED because both sides of this statement are the monsters
-- table: an unqualified `id` in the WHERE is ambiguous between the row being
-- written and the row being read, and MySQL refuses it rather than guessing.
--
-- asset_id IS DELIBERATELY NOT AMONG THEM. It names an object under the source
-- owner's own prefix, and a row in this account pointing there would break the
-- moment they deleted their monster. The picture is copied as a new object and
-- linked afterwards, by the same attachMonsterImage every upload goes through.
INSERT INTO monsters (
    id, owner_id,
    name, size, `type`, tags, alignment,
    ac, hp, hit_dice, speed, initiative_bonus, cr,
    legendary_action_uses, legendary_action_uses_in_lair,
    `str`, dex, `con`, `int`, wis, cha,
    skills, skill_proficiencies, saving_throws, saving_throw_proficiencies,
    vulnerabilities, resistances, immunities, gear, senses, languages,
    habitat, treasure, description
)
SELECT sqlc.arg(id), sqlc.arg(owner_id),
    original.name, original.size, original.`type`, original.tags, original.alignment,
    original.ac, original.hp, original.hit_dice, original.speed, original.initiative_bonus, original.cr,
    original.legendary_action_uses, original.legendary_action_uses_in_lair,
    original.`str`, original.dex, original.`con`, original.`int`, original.wis, original.cha,
    original.skills, original.skill_proficiencies, original.saving_throws, original.saving_throw_proficiencies,
    original.vulnerabilities, original.resistances, original.immunities, original.gear, original.senses, original.languages,
    original.habitat, original.treasure, original.description
FROM monsters original
WHERE original.id = sqlc.arg(source_id) AND original.owner_id = sqlc.arg(source_owner_id);

-- name: UpdateMonsterImage :exec
UPDATE monsters
SET asset_id = ?
WHERE id = ? AND owner_id = ?;

-- name: DeleteMonster :exec
DELETE FROM monsters
WHERE id = ? AND owner_id = ?;

-- THE PANEL UPDATES.
--
-- The editor autosaves one panel at a time, so each statement writes the
-- columns its panel owns and nothing else. The reason is the character sheet's
-- and it has not changed: the parse helpers return their fallback on an empty
-- string rather than an error, so a statement wider than the panel that posted
-- to it would not fail -- it would write 10s over the ability scores and empty
-- objects over the bonus grids, and report success.
--
-- TestMonsterPanelsCoverEveryEditableColumn reads the schema dump and holds the
-- other half of that line: every column here belongs to exactly one panel, and
-- the five it does not name -- id, owner_id, asset_id, created_at, updated_at --
-- belong to none.
--
-- All are :execresult because the pool runs with found-rows semantics: zero
-- matched rows means "not this user's monster" rather than "nothing changed",
-- so the handler answers 404 instead of a false success.

-- name: UpdateMonsterIdentity :execresult
UPDATE monsters
SET
    name = ?,
    size = ?,
    `type` = ?,
    tags = ?,
    alignment = ?
WHERE id = ? AND owner_id = ?;

-- name: UpdateMonsterAbilities :execresult
UPDATE monsters
SET
    `str` = ?,
    dex = ?,
    `con` = ?,
    `int` = ?,
    wis = ?,
    cha = ?
WHERE id = ? AND owner_id = ?;

-- Combat is the panel the block's top three lines come off, plus the two
-- numbers behind the legendary sentence. Nothing here is derived: the
-- proficiency bonus and the XP follow from cr and are worked out at render
-- time, and the initiative the block prints is dex plus initiative_bonus. A
-- stored copy of either would be correct until the Abilities panel saved.
--
-- The legendary counts sit here rather than beside the legendary action rows
-- because those rows are each their own form and forms cannot nest -- the same
-- reason the character sheet keeps nothing of the Attacks panel in a form
-- around it.

-- name: UpdateMonsterCombat :execresult
UPDATE monsters
SET
    ac = ?,
    hp = ?,
    hit_dice = ?,
    speed = ?,
    initiative_bonus = ?,
    cr = ?,
    legendary_action_uses = ?,
    legendary_action_uses_in_lair = ?
WHERE id = ? AND owner_id = ?;

-- name: UpdateMonsterDefenses :execresult
UPDATE monsters
SET
    vulnerabilities = ?,
    resistances = ?,
    immunities = ?,
    gear = ?,
    senses = ?,
    languages = ?
WHERE id = ? AND owner_id = ?;

-- The two bonus grids, and they are the character sheet's grids: the same
-- components post them, so the same two columns come back -- the misc bonus per
-- row and the proficiency state per row. Both halves of a grid move in one
-- statement because neither means anything without the other, and a debounce
-- landing between two statements would leave a row proficient with somebody
-- else's misc bonus.

-- name: UpdateMonsterSkills :execresult
UPDATE monsters
SET skills = ?, skill_proficiencies = ?
WHERE id = ? AND owner_id = ?;

-- name: UpdateMonsterSavingThrows :execresult
UPDATE monsters
SET saving_throws = ?, saving_throw_proficiencies = ?
WHERE id = ? AND owner_id = ?;

-- habitat and treasure are the two lines the book prints under the block, and
-- description is the GM's own paragraph. They share a panel because they are
-- the three fields nobody rolls.

-- name: UpdateMonsterDescription :execresult
UPDATE monsters
SET
    habitat = ?,
    treasure = ?,
    description = ?
WHERE id = ? AND owner_id = ?;
