-- Every query here is scoped to the owner. A character id from someone else's
-- roster matches nothing, so a handler never has to check ownership after
-- the fact: no row means not yours or not there, and both answer 404.

-- name: GetCharacters :many
SELECT * FROM characters
WHERE owner_id = ?
ORDER BY created_at DESC;

-- name: GetCharacter :one
SELECT * FROM characters
WHERE id = ? AND owner_id = ?;

-- name: GetCharacterAsset :one
SELECT c.name, c.asset_id, a.file_path FROM characters c
LEFT JOIN assets a ON a.id = c.asset_id
WHERE c.id = ? AND c.owner_id = ?;

-- name: DeleteCharacter :exec
DELETE FROM characters
WHERE id = ? AND owner_id = ?;

-- CreateCharacterFromName is the whole of character creation. The modal asks for
-- a name and nothing else; every other column is answered here or by the
-- schema, and the sheet is filled in afterwards by the editor, which autosaves.
--
-- Three parameters is the point of it. A statement that took the sheet would be
-- a second wide writer, and the reason there is not one is written at the top of
-- the panel updates below. Creation cannot drift into that shape without growing
-- a parameter, which is what the params-shape test refuses.
--
-- The literals are the twelve NOT NULL columns the schema has no DEFAULT for
-- plus spell_save_dc, which has one that disagrees. Its default is 0 and the old
-- create form started it at 10; naming it keeps a new character where it was.
-- The rest -- level, xp, size, ac, max_hp, current_hp, proficiency_bonus,
-- temp_hp, initiative_bonus, spell_atk_bonus -- have DEFAULTs that already match
-- what that form produced, so they are left to the table.
--
-- The blobs go in empty rather than pre-shaped. parseStatBonuses and
-- parseFeatures each return their empty shape for a blob that carries nothing,
-- so a fresh sheet reads back exactly as the blank form renders. Spell slots are
-- not among them any more: they are rows in their own table, created the first
-- time a level is given a count, and a character that has never cast anything
-- has none.
--
-- race, background, alignment and classes are nullable and stay NULL;
-- characterToEditPageData already renders that as its fallback text.

-- name: CreateCharacterFromName :exec
INSERT INTO characters (
    id,
    owner_id,
    name,
    `str`,
    dex,
    `con`,
    `int`,
    wis,
    cha,
    speed,
    languages,
    proficiencies,
    skills,
    saving_throws,
    features,
    spell_save_dc
) VALUES (
    ?, ?, ?,
    10, 10, 10, 10, 10, 10,
    '30 ft.', '', '',
    '{}', '{}', '[]',
    10
);

-- name: UpdateCharacterAvatar :exec
UPDATE characters
SET asset_id = ?
WHERE id = ? AND owner_id = ?;

-- THE PANEL UPDATES.
--
-- The character editor autosaves one panel at a time, so each of these writes
-- only the columns its panel owns and nothing else. That narrowness is the
-- whole point and it is not a style choice: the parse helpers in the controller
-- return their fallback on an empty string rather than an error, so a partial
-- post handled by a statement wider than the panel that sent it would not fail
-- -- it would silently write 10s over the ability scores, 1s over the hit
-- points and empty JSON over all six blobs. So a panel's columns and its
-- statement's columns are the same set, and nothing writes a superset.
--
-- All are :execresult because the pool runs with found-rows semantics: zero
-- matched rows means "not this user's character", not "nothing changed", so the
-- handler can answer 404 rather than a false success.

-- name: UpdateCharacterIdentity :execresult
UPDATE characters
SET
    name = ?,
    race = ?,
    background = ?,
    alignment = ?,
    classes = ?,
    size = ?
WHERE id = ? AND owner_id = ?;

-- name: UpdateCharacterAbilities :execresult
UPDATE characters
SET
    `str` = ?,
    dex = ?,
    `con` = ?,
    `int` = ?,
    wis = ?,
    cha = ?
WHERE id = ? AND owner_id = ?;

-- level and proficiency_bonus are not fields on the form. They are derived from
-- xp by levelFromXP and proficiencyBonusForLevel, and xp lives in this panel, so
-- they are recomputed in the handler and written here in the same statement.
-- Splitting them out would leave a window where a row's level disagreed with its
-- xp.
-- name: UpdateCharacterCoreStats :execresult
UPDATE characters
SET
    xp = ?,
    level = ?,
    proficiency_bonus = ?,
    speed = ?,
    ac = ?,
    initiative_bonus = ?,
    max_hp = ?,
    current_hp = ?,
    temp_hp = ?,
    spell_save_dc = ?,
    spell_atk_bonus = ?
WHERE id = ? AND owner_id = ?;

-- name: UpdateCharacterProficiencies :execresult
UPDATE characters
SET
    languages = ?,
    proficiencies = ?
WHERE id = ? AND owner_id = ?;

-- The two halves of the sheet nobody rolls. They are split because their inputs
-- are: four boxes of prose, and six words. One panel of ten would have been one
-- statement of ten, which is still narrow, but the page reads better as two and
-- the split costs a query.

-- name: UpdateCharacterPersonality :execresult
UPDATE characters
SET
    personality_traits = ?,
    ideals = ?,
    bonds = ?,
    flaws = ?
WHERE id = ? AND owner_id = ?;

-- name: UpdateCharacterAppearance :execresult
UPDATE characters
SET
    age = ?,
    height = ?,
    weight = ?,
    eyes = ?,
    skin = ?,
    hair = ?
WHERE id = ? AND owner_id = ?;

-- The six single-column writes below back the bonus tables and the three
-- repeaters. Each takes the JSON its panel posts, already marshalled by the
-- controller.

-- name: UpdateCharacterSkills :execresult
UPDATE characters
SET skills = ?
WHERE id = ? AND owner_id = ?;

-- name: UpdateCharacterSavingThrows :execresult
UPDATE characters
SET saving_throws = ?
WHERE id = ? AND owner_id = ?;

-- name: UpdateCharacterFeatures :execresult
UPDATE characters
SET features = ?
WHERE id = ? AND owner_id = ?;
