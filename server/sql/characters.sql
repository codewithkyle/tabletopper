-- name: GetCharacters :many
SELECT id, name, level, race, classes, ac, current_hp, asset_id FROM characters
WHERE owner_id = ?
ORDER BY created_at DESC;

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
