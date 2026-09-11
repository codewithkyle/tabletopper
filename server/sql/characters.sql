-- name: GetCharacters :many
SELECT * FROM characters
WHERE owner_id = ?
ORDER BY created_at DESC;

-- name: GetCharacter :one
SELECT * FROM characters
WHERE id = ? AND owner_id = ?;

-- name: GetCharacterName :one
SELECT name FROM characters
WHERE id = ? AND owner_id = ?;

-- name: GetCharacterAsset :one
SELECT c.name, c.asset_id, a.file_path FROM characters c
LEFT JOIN assets a ON a.id = c.asset_id
WHERE c.id = ? AND c.owner_id = ?;

-- name: DeleteCharacter :exec
DELETE FROM characters
WHERE id = ? AND owner_id = ?;

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

-- name: GetCharacterForRoom :one
SELECT id, owner_id, name, size, ac, current_hp, max_hp, asset_id FROM characters
WHERE id = ?;

-- name: UpdateCharacterCurrentHP :execresult
UPDATE characters
SET current_hp = ?
WHERE id = ?;
