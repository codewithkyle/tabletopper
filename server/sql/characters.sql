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

-- UpdateCharacter is :execresult so the handler can tell a character that is
-- not the caller's (no row matched, answer 404) from one that saved. The pool
-- is opened with found-rows semantics, so a save that changed nothing still
-- counts as matched.
-- name: UpdateCharacter :execresult
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
