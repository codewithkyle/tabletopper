-- migrate:up
-- THE THIRD MEMBER OF resource_type, and the first one that is not a
-- character's. A shared monster is a row in this table for the reason a shared
-- character sheet is: it inherits the token, the password, the expiry, the
-- sweep and the one-live-link-per-thing rule without any of them being written
-- a second time.
--
-- resource_id FOR A MONSTER SHARE IS THE MONSTER'S OWN ID, the way a character
-- share's is the character's. ux_shares_resource is what turns that into "one
-- live link per monster", and it needs nothing added to do it.
--
-- character_id BECOMES NULLABLE, WHICH IS THE WHOLE OF THE COST. That column
-- was never the thing being shared -- it is the parent a share hangs off, so
-- that deleting a character can find and delete these rows without a subquery
-- against the journals table the same request is emptying. A monster hangs off
-- no character. It belongs to the account directly, the way the manual does,
-- so there is no parent to name and NULL is what that says.
--
-- The alternative was writing the monster's own id into character_id, which
-- would have been free -- no ULID a character has can collide with one a
-- monster has, so DeleteSharesForCharacter would never have matched a monster
-- share by accident. It is refused because the column would then be lying
-- about what it holds, and the next person reading `idx_shares_character` or
-- joining on it has no way to find that out.
--
-- WHAT SWEEPS A MONSTER SHARE INSTEAD IS resource_id. DeleteMonsterShare pins
-- resource_type and names the monster, which is the same statement the revoke
-- button runs -- a monster has exactly one link, so deleting the monster and
-- revoking its link are the same delete, and there is no DeleteSharesForMonster
-- beside it the way there is for a character.
ALTER TABLE shares
    MODIFY resource_type ENUM('journal', 'character', 'monster') NOT NULL DEFAULT 'journal',
    MODIFY character_id VARBINARY(16) NULL;

-- migrate:down
-- THE ROWS GO BEFORE THE COLUMN AND THE ENUM DO, for the reason 20260906260000
-- gives: narrowing an ENUM that still has rows in the member being dropped does
-- not fail, it rewrites them to the empty string and leaves shares that match
-- no branch of the reader, cannot be opened, and cannot be revoked from any
-- dialog either. It is also what makes character_id NOT NULL again possible --
-- every remaining row is a journal's or a character's, and both carry one.
DELETE FROM shares WHERE resource_type = 'monster';

ALTER TABLE shares
    MODIFY character_id VARBINARY(16) NOT NULL,
    MODIFY resource_type ENUM('journal', 'character') NOT NULL DEFAULT 'journal';
