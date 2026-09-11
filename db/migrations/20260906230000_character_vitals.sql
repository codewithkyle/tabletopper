-- migrate:up
ALTER TABLE characters
    ADD COLUMN hit_dice VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN hit_dice_spent TINYINT UNSIGNED NOT NULL DEFAULT 0,
    ADD COLUMN death_save_successes TINYINT UNSIGNED NOT NULL DEFAULT 0,
    ADD COLUMN death_save_failures TINYINT UNSIGNED NOT NULL DEFAULT 0,
    ADD COLUMN heroic_inspiration TINYINT(1) NOT NULL DEFAULT 0,
    ADD COLUMN exhaustion TINYINT UNSIGNED NOT NULL DEFAULT 0,
    ADD CONSTRAINT chk_characters_hit_dice_spent CHECK (hit_dice_spent <= 20),
    ADD CONSTRAINT chk_characters_death_saves CHECK (death_save_successes <= 3 AND death_save_failures <= 3),
    ADD CONSTRAINT chk_characters_exhaustion CHECK (exhaustion <= 6);

-- migrate:down
ALTER TABLE characters
    DROP CONSTRAINT chk_characters_exhaustion,
    DROP CONSTRAINT chk_characters_death_saves,
    DROP CONSTRAINT chk_characters_hit_dice_spent,
    DROP COLUMN exhaustion,
    DROP COLUMN heroic_inspiration,
    DROP COLUMN death_save_failures,
    DROP COLUMN death_save_successes,
    DROP COLUMN hit_dice_spent,
    DROP COLUMN hit_dice;
