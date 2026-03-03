-- name: GetCharacters :many
SELECT
    id,
    name,
    level,
    xp,
    race,
    classes,
    background,
    alignment,
    size,
    ac,
    max_hp,
    current_hp,
    proficiency_bonus,
    speed,
    asset_id
FROM characters
WHERE owner_id = ?
ORDER BY created_at DESC;

-- name: GetCharacterByIDAndOwner :one
SELECT * FROM characters
WHERE id = ? AND owner_id = ?;

-- name: GetCharacterAssetByIDAndOwner :one
SELECT c.name, c.asset_id, a.file_path FROM characters c
LEFT JOIN assets a ON a.id = c.asset_id
WHERE c.id = ? AND c.owner_id = ?;

-- name: DeleteCharacterByIDAndOwner :exec
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

-- name: UpdateCharacterByIDAndOwner :execresult
UPDATE characters
SET
    name = ?,
    level = ?,
    xp = ?,
    race = ?,
    background = ?,
    alignment = ?,
    classes = ?,
    size = ?,
    ac = ?,
    max_hp = ?,
    current_hp = ?,
    proficiency_bonus = ?,
    temp_hp = ?,
    speed = ?,
    initiative_bonus = ?,
    spell_save_dc = ?,
    spell_atk_bonus = ?,
    `str` = ?,
    dex = ?,
    `con` = ?,
    `int` = ?,
    wis = ?,
    cha = ?,
    languages = ?,
    proficiencies = ?,
    skills = ?,
    saving_throws = ?,
    features = ?,
    weapons = ?,
    spell_slots = ?,
    resources = ?
WHERE id = ? AND owner_id = ?;

-- name: UpdateCharacterAvatar :exec
UPDATE characters
SET asset_id = ?
WHERE id = ? AND owner_id = ?;
