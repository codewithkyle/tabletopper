-- migrate:up
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
