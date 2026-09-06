-- migrate:up
-- The second member of resource_type, which is the reason the column was an
-- enum of one. A shared character sheet is a row in this table rather than a
-- table of its own, so it inherits the token, the password, the expiry, the
-- sweep and the one-live-link-per-thing rule without any of them being written
-- a second time.
--
-- resource_id FOR A CHARACTER SHARE IS THE CHARACTER'S OWN ID, which is the
-- value character_id already holds. That is two columns doing two jobs with
-- one value rather than a redundancy to tidy away. resource_id is half of
-- ux_shares_resource, so it is what makes "one live link per character" the
-- database's rule; character_id is what DeleteCharacterShares finds these rows
-- by when the character is deleted, and what the schema-driven purge test
-- looks for. Collapsing either into the other would cost one of those.
ALTER TABLE shares
    MODIFY resource_type ENUM('journal', 'character') NOT NULL DEFAULT 'journal';

-- migrate:down
-- THE ROWS GO BEFORE THE ENUM DOES. Narrowing an ENUM that still has rows in
-- the member being dropped does not fail -- MySQL rewrites them to the empty
-- string and reports a warning -- which would leave shares that match no
-- branch of the reader, cannot be opened, and cannot be revoked from any
-- dialog either, because every owner statement names its own type.
DELETE FROM shares WHERE resource_type = 'character';

ALTER TABLE shares
    MODIFY resource_type ENUM('journal') NOT NULL DEFAULT 'journal';
