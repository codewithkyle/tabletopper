-- migrate:up
-- Six more columns on characters, for the reason the details ten are columns:
-- each is one scalar, each is 1:1 with the character, and each has a default
-- that is right for a row nobody has touched. These differ from those only in
-- how often they change -- this is the half of the sheet that moves during a
-- fight -- and that is not a reason to put them anywhere else.
--
-- HIT DICE IS TEXT AND SPENT IS A NUMBER, because nothing here can derive the
-- pool. The die follows from the class and classes is a free-text note on this
-- sheet ("Cleric 3, Fighter 1"), while level follows from xp -- so a multiclass
-- character's 3d8 + 1d10 is a sentence this app cannot compose and the player
-- can. What it can count is how many have been spent, and that is the number
-- that changes at every short rest.
--
-- THE CHECKS ARE THE DOMAIN, not a second opinion about the handler. Death
-- saves are three bubbles each in the rules; exhaustion runs 0 to 6, where 6 is
-- death; and a character can never hold more hit dice than levels, which
-- levelFromXP caps at 20. buildVitalsInput refuses anything outside those before
-- a write is attempted, so a constraint firing here means a bug in that
-- function rather than a value a player produced -- which is the point of
-- having both.
--
-- heroic_inspiration is a tinyint(1) like inventory.equipped and
-- spells.is_prepared. It is one bit, and an unticked box posts nothing at all,
-- so the panel that owns it has to render every one of its controls together --
-- a partial post would read as inspiration spent. The Vitals panel does, and a
-- test pins it.
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
