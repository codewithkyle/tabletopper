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
-- The literals are the twelve NOT NULL columns the schema has no DEFAULT for.
-- spell_save_dc used to be a thirteenth, named because its default disagreed
-- with what the old create form produced; the derivation pass dropped it, and
-- the spellcasting columns that replaced it default to a character who casts
-- nothing, which is the right starting point and needs no literal here.
--
-- The rest -- level, xp, size, ac, max_hp, current_hp, proficiency_bonus,
-- temp_hp, initiative_bonus -- have DEFAULTs that already match what that form
-- produced, so they are left to the table. So do the two proficiency blobs,
-- which start as empty objects the way skills and saving_throws are written
-- empty below.
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
    features
) VALUES (
    ?, ?, ?,
    10, 10, 10, 10, 10, 10,
    '30 ft.', '', '',
    '{}', '{}', '[]'
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
--
-- The three hit point columns were here until the vitals panel existed to take
-- them. They belong beside the death saves and the hit dice, which are the other
-- numbers that move when a character is hurt, rather than beside the speed and
-- the experience, which are not.
-- name: UpdateCharacterCoreStats :execresult
UPDATE characters
SET
    xp = ?,
    level = ?,
    proficiency_bonus = ?,
    speed = ?,
    ac = ?,
    initiative_bonus = ?,
    spellcasting_ability = ?,
    spell_bonus_misc = ?
WHERE id = ? AND owner_id = ?;

-- name: UpdateCharacterProficiencies :execresult
UPDATE characters
SET
    languages = ?,
    proficiencies = ?
WHERE id = ? AND owner_id = ?;

-- Vitals is the counterpart to core stats: the numbers on that panel describe
-- what a character is, and these nine describe how the last fight went. They are
-- split for that reason and for a mechanical one -- core stats derives level and
-- proficiency from xp on every save, and nothing here should be recomputed by a
-- player ticking a death save.
--
-- The hit points lead because they are what the panel is opened for. They also
-- explain the rest of it: a character reaches the death saves by running out of
-- them and gets some back by spending a hit die.

-- name: UpdateCharacterVitals :execresult
UPDATE characters
SET
    max_hp = ?,
    current_hp = ?,
    temp_hp = ?,
    hit_dice = ?,
    hit_dice_spent = ?,
    death_save_successes = ?,
    death_save_failures = ?,
    heroic_inspiration = ?,
    exhaustion = ?
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

-- The writes below back the bonus grids and the features repeater. Each takes
-- the JSON its panel posts, already marshalled by the controller.
--
-- The two bonus grids write TWO columns each since the derivation pass: the misc
-- bonuses they always held, and the proficiency state each row is now set from.
-- Both come off the same form and neither means anything without the other, so
-- splitting them across two statements would let a debounce land between them
-- and leave a row proficient with somebody else's misc bonus.

-- name: UpdateCharacterSkills :execresult
UPDATE characters
SET skills = ?, skill_proficiencies = ?
WHERE id = ? AND owner_id = ?;

-- name: UpdateCharacterSavingThrows :execresult
UPDATE characters
SET saving_throws = ?, saving_throw_proficiencies = ?
WHERE id = ? AND owner_id = ?;

-- name: UpdateCharacterFeatures :execresult
UPDATE characters
SET features = ?
WHERE id = ? AND owner_id = ?;
