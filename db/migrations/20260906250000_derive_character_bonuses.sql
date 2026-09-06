-- migrate:up
-- THE DERIVATION PASS. Until now every bonus on this sheet was a number somebody
-- typed: a skill bonus, a saving throw bonus, a spell save DC. That means the
-- player did the arithmetic, and did it again at every proficiency bump, every
-- ability score increase and every level -- eighteen skills and six saves at a
-- time, from three inputs the sheet already knew.
--
-- What is stored after this is what the rules call an INPUT, and every total is
-- computed where it is rendered. A skill bonus is the ability modifier plus what
-- the proficiency state grants plus a misc bonus, and only the last two are
-- columns. Nothing derived is stored at all, and that is not a preference: a
-- derived value depends on columns two different panels own, and this editor
-- autosaves a panel at a time. Stored, it would go stale the moment the OTHER
-- panel saved -- and the fix, letting both panels write it, is the exact thing
-- TestPanelsCoverEveryEditableColumn refuses.
--
-- skills and saving_throws KEEP THEIR NAME AND CHANGE THEIR MEANING: each held a
-- total and now holds the misc part of one. The UPDATE below is what makes that
-- lossless -- it subtracts the ability modifier the total already contained, so
-- every sheet renders the same number after this migration as before it. What it
-- cannot do is guess which part of that number was proficiency: a player who had
-- +7 stealth keeps +7, and the day they tick Proficient they will want to clear
-- the misc that is now carrying it. Inferring it here would mean writing a
-- five-branch CASE eighteen times against a proficiency bonus that is itself
-- derived, to save a one-off edit on a handful of rows.
--
-- CAST(... AS SIGNED) IS NOT DECORATION. The ability columns are TINYINT
-- UNSIGNED, and MySQL makes the whole expression unsigned if either operand is
-- -- so `dex DIV 2 - 5` on a Dex of 8 does not produce -1, it raises "BIGINT
-- UNSIGNED value is out of range" and fails the migration.
--
-- spell_save_dc and spell_atk_bonus go, replaced by the ability that produces
-- them and a misc bonus for the items that adjust them. The misc is set from the
-- ATTACK bonus rather than the DC because the two are related -- a save DC is
-- 8 + the attack bonus -- so a sheet that was internally consistent keeps both
-- numbers, and one that was not moves to the consistent pair.
ALTER TABLE characters
    ADD COLUMN skill_proficiencies JSON NOT NULL DEFAULT (JSON_OBJECT()),
    ADD COLUMN saving_throw_proficiencies JSON NOT NULL DEFAULT (JSON_OBJECT()),
    ADD COLUMN spellcasting_ability ENUM('none', 'str', 'dex', 'con', 'int', 'wis', 'cha') NOT NULL DEFAULT 'none',
    ADD COLUMN spell_bonus_misc SMALLINT NOT NULL DEFAULT 0;

UPDATE characters
SET
    skills = JSON_OBJECT(
        'acrobatics', COALESCE(JSON_VALUE(skills, '$.acrobatics' RETURNING SIGNED), 0)
            - (CAST(dex AS SIGNED) DIV 2 - 5),
        'animal_handling', COALESCE(JSON_VALUE(skills, '$.animal_handling' RETURNING SIGNED), 0)
            - (CAST(wis AS SIGNED) DIV 2 - 5),
        'arcana', COALESCE(JSON_VALUE(skills, '$.arcana' RETURNING SIGNED), 0)
            - (CAST(`int` AS SIGNED) DIV 2 - 5),
        'athletics', COALESCE(JSON_VALUE(skills, '$.athletics' RETURNING SIGNED), 0)
            - (CAST(`str` AS SIGNED) DIV 2 - 5),
        'deception', COALESCE(JSON_VALUE(skills, '$.deception' RETURNING SIGNED), 0)
            - (CAST(cha AS SIGNED) DIV 2 - 5),
        'history', COALESCE(JSON_VALUE(skills, '$.history' RETURNING SIGNED), 0)
            - (CAST(`int` AS SIGNED) DIV 2 - 5),
        'insight', COALESCE(JSON_VALUE(skills, '$.insight' RETURNING SIGNED), 0)
            - (CAST(wis AS SIGNED) DIV 2 - 5),
        'intimidation', COALESCE(JSON_VALUE(skills, '$.intimidation' RETURNING SIGNED), 0)
            - (CAST(cha AS SIGNED) DIV 2 - 5),
        'investigation', COALESCE(JSON_VALUE(skills, '$.investigation' RETURNING SIGNED), 0)
            - (CAST(`int` AS SIGNED) DIV 2 - 5),
        'medicine', COALESCE(JSON_VALUE(skills, '$.medicine' RETURNING SIGNED), 0)
            - (CAST(wis AS SIGNED) DIV 2 - 5),
        'nature', COALESCE(JSON_VALUE(skills, '$.nature' RETURNING SIGNED), 0)
            - (CAST(`int` AS SIGNED) DIV 2 - 5),
        'perception', COALESCE(JSON_VALUE(skills, '$.perception' RETURNING SIGNED), 0)
            - (CAST(wis AS SIGNED) DIV 2 - 5),
        'performance', COALESCE(JSON_VALUE(skills, '$.performance' RETURNING SIGNED), 0)
            - (CAST(cha AS SIGNED) DIV 2 - 5),
        'persuasion', COALESCE(JSON_VALUE(skills, '$.persuasion' RETURNING SIGNED), 0)
            - (CAST(cha AS SIGNED) DIV 2 - 5),
        'religion', COALESCE(JSON_VALUE(skills, '$.religion' RETURNING SIGNED), 0)
            - (CAST(`int` AS SIGNED) DIV 2 - 5),
        'sleight_of_hand', COALESCE(JSON_VALUE(skills, '$.sleight_of_hand' RETURNING SIGNED), 0)
            - (CAST(dex AS SIGNED) DIV 2 - 5),
        'stealth', COALESCE(JSON_VALUE(skills, '$.stealth' RETURNING SIGNED), 0)
            - (CAST(dex AS SIGNED) DIV 2 - 5),
        'survival', COALESCE(JSON_VALUE(skills, '$.survival' RETURNING SIGNED), 0)
            - (CAST(wis AS SIGNED) DIV 2 - 5)
    ),
    saving_throws = JSON_OBJECT(
        'str', COALESCE(JSON_VALUE(saving_throws, '$.str' RETURNING SIGNED), 0)
            - (CAST(`str` AS SIGNED) DIV 2 - 5),
        'dex', COALESCE(JSON_VALUE(saving_throws, '$.dex' RETURNING SIGNED), 0)
            - (CAST(dex AS SIGNED) DIV 2 - 5),
        'con', COALESCE(JSON_VALUE(saving_throws, '$.con' RETURNING SIGNED), 0)
            - (CAST(`con` AS SIGNED) DIV 2 - 5),
        'int', COALESCE(JSON_VALUE(saving_throws, '$.int' RETURNING SIGNED), 0)
            - (CAST(`int` AS SIGNED) DIV 2 - 5),
        'wis', COALESCE(JSON_VALUE(saving_throws, '$.wis' RETURNING SIGNED), 0)
            - (CAST(wis AS SIGNED) DIV 2 - 5),
        'cha', COALESCE(JSON_VALUE(saving_throws, '$.cha' RETURNING SIGNED), 0)
            - (CAST(cha AS SIGNED) DIV 2 - 5)
    ),
    spell_bonus_misc = spell_atk_bonus - CAST(proficiency_bonus AS SIGNED);

ALTER TABLE characters
    DROP COLUMN spell_save_dc,
    DROP COLUMN spell_atk_bonus;

-- migrate:down
-- Down restores the shape, not the arithmetic. The totals cannot be rebuilt
-- without the proficiency states this drops, so the misc bonuses go back as the
-- totals they will be read as, and a sheet that had used a proficiency toggle
-- comes back lower than it went in.
ALTER TABLE characters
    ADD COLUMN spell_save_dc SMALLINT UNSIGNED NOT NULL DEFAULT 0,
    ADD COLUMN spell_atk_bonus SMALLINT NOT NULL DEFAULT 0;

UPDATE characters
SET
    spell_atk_bonus = spell_bonus_misc + CAST(proficiency_bonus AS SIGNED),
    spell_save_dc = 8 + spell_bonus_misc + CAST(proficiency_bonus AS SIGNED);

ALTER TABLE characters
    DROP COLUMN spell_bonus_misc,
    DROP COLUMN spellcasting_ability,
    DROP COLUMN saving_throw_proficiencies,
    DROP COLUMN skill_proficiencies;
