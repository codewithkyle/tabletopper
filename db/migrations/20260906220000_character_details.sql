-- migrate:up
-- Ten columns on characters rather than a character_details table, for the
-- reason the preferences columns sit on users: each is one scalar, each is 1:1
-- with the character, and every one has a default that is right for a row
-- nobody has filled in -- so there is no backfill, no NULL to handle, and no
-- join on the read path.
--
-- THE FOUR PROSE COLUMNS ARE TEXT AND THE SIX APPEARANCE COLUMNS ARE VARCHAR,
-- and the line between them is whether the answer is a sentence or a word. TEXT
-- is stored off-page, so a bond somebody writes an essay into costs the row a
-- pointer rather than its whole length. The appearance six answer "what colour
-- are their eyes", and 64 characters is already more room than that question
-- has ever needed. It is also what race and background take, which is the same
-- kind of value.
--
-- THIS IS NOT characters.notes COMING BACK. That column was dropped on
-- 2026-09-05 because nothing read it and the journals table was what it had
-- been reaching for. These ten are each rendered and written by a panel, and
-- none of them is a place to keep a diary -- the journal still is, and
-- backstory belongs there rather than in an eleventh column here.
--
-- Every one is NOT NULL DEFAULT '' rather than nullable. race, background,
-- alignment and classes are nullable, and every read of them runs through
-- nullStringValue or characterValueOrFallback to fold NULL and '' back into one
-- empty string. Two spellings of "not filled in" is one more than this sheet
-- has ever needed.
--
-- TEXT takes only an expression default, which is why the first four carry ('')
-- where the last six carry ''.
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
