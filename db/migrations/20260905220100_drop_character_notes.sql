-- migrate:up
-- characters.notes was written as '' by CreateCharacterFromName and read by
-- nothing -- the panel-coverage test listed it among the columns no panel owns.
-- The journals table is what it was reaching for, and journal entries have a
-- title, their own timestamps and one row each, none of which a single TEXT
-- column on the character was ever going to grow.
--
-- Nothing salvages the column: it has only ever held the empty string.
ALTER TABLE characters DROP COLUMN notes;

-- migrate:down
-- Back empty, and with a default the original did not have -- NOT NULL cannot be
-- added to a populated table without one, and TEXT takes only an expression
-- default. Down restores the shape, not the contents.
ALTER TABLE characters ADD COLUMN notes TEXT NOT NULL DEFAULT ('');
