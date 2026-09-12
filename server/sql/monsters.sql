-- name: ListMonsters :many
SELECT * FROM monsters
WHERE owner_id = ?
ORDER BY name;
-- name: SearchMonsters :many
SELECT * FROM monsters
WHERE owner_id = sqlc.arg(owner_id) AND name LIKE sqlc.arg(term)
ORDER BY name;
-- name: GetMonster :one
SELECT * FROM monsters
WHERE id = ? AND owner_id = ?;
-- name: GetMonsterAsset :one
SELECT m.name, m.asset_id, a.file_path FROM monsters m
LEFT JOIN assets a ON a.id = m.asset_id
WHERE m.id = ? AND m.owner_id = ?;
-- name: CreateMonsterFromName :exec
INSERT INTO monsters (id, owner_id, name)
VALUES (?, ?, ?);
-- name: CreateQuickMonster :exec
INSERT INTO monsters (id, owner_id, name, size, ac, hp)
VALUES (?, ?, ?, ?, ?, ?);
-- name: CopyMonster :execresult
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
-- name: UpdateMonsterSkills :execresult
UPDATE monsters
SET skills = ?, skill_proficiencies = ?
WHERE id = ? AND owner_id = ?;
-- name: UpdateMonsterSavingThrows :execresult
UPDATE monsters
SET saving_throws = ?, saving_throw_proficiencies = ?
WHERE id = ? AND owner_id = ?;
-- name: UpdateMonsterDescription :execresult
UPDATE monsters
SET
    habitat = ?,
    treasure = ?,
    description = ?
WHERE id = ? AND owner_id = ?;
-- name: GetMonsterForRoom :one
SELECT id, name, size, ac, hp, initiative_bonus, asset_id FROM monsters
WHERE id = ? AND owner_id = ?;
