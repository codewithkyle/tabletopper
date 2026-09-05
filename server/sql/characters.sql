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

-- name: CreateCharacter :exec
INSERT INTO characters (
    id,
    owner_id,
    asset_id,
    name,
    level,
    xp,
    race,
    background,
    alignment,
    classes,
    size,
    ac,
    max_hp,
    current_hp,
    proficiency_bonus,
    temp_hp,
    speed,
    initiative_bonus,
    spell_save_dc,
    spell_atk_bonus,
    `str`,
    dex,
    `con`,
    `int`,
    wis,
    cha,
    languages,
    proficiencies,
    skills,
    saving_throws,
    features,
    weapons,
    spell_slots,
    resources,
    notes
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
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

-- name: UpdateCharacterWeapons :execresult
UPDATE characters
SET weapons = ?
WHERE id = ? AND owner_id = ?;

-- name: UpdateCharacterResources :execresult
UPDATE characters
SET resources = ?
WHERE id = ? AND owner_id = ?;

-- name: UpdateCharacterSpellSlots :execresult
UPDATE characters
SET spell_slots = ?
WHERE id = ? AND owner_id = ?;
