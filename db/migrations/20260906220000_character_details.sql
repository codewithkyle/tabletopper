-- migrate:up
ALTER TABLE characters
    ADD COLUMN personality_traits TEXT NOT NULL DEFAULT (''),
    ADD COLUMN ideals TEXT NOT NULL DEFAULT (''),
    ADD COLUMN bonds TEXT NOT NULL DEFAULT (''),
    ADD COLUMN flaws TEXT NOT NULL DEFAULT (''),
    ADD COLUMN age VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN height VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN weight VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN eyes VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN skin VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN hair VARCHAR(64) NOT NULL DEFAULT '';

-- migrate:down
ALTER TABLE characters
    DROP COLUMN hair,
    DROP COLUMN skin,
    DROP COLUMN eyes,
    DROP COLUMN weight,
    DROP COLUMN height,
    DROP COLUMN age,
    DROP COLUMN flaws,
    DROP COLUMN bonds,
    DROP COLUMN ideals,
    DROP COLUMN personality_traits;
