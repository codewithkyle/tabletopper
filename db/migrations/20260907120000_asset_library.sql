-- migrate:up
-- THE ASSET MANAGER GROWS A LIBRARY, AND `avatar` IS THE MEMBER IT NEEDS. Until
-- now `avatar` meant one thing only: the portrait a character carries, written
-- by InsertAvatar and pointed at by characters.asset_id. The Avatars page in the
-- asset manager is a different thing entirely -- stock a game master gathers so
-- that spawning an NPC mid-session is a search rather than a file dialog -- and
-- the two cannot share a member.
--
-- THE REASON IS DELETION, NOT TIDINESS. A page listing `type = 'avatar'` offers
-- a Delete on every row it shows, and there is not a foreign key anywhere in
-- this schema: characters.asset_id is a plain nullable VARBINARY(16). Deleting a
-- portrait from the library page would leave a character pointing at a row that
-- no longer exists, the image route would answer 404, and the only symptom would
-- be a portrait that had quietly become a broken picture. Nothing would fail and
-- nothing would log.
--
-- SO THE PORTRAIT MOVES AND THE LIBRARY KEEPS THE NAME. The axis these members
-- divide on is what owns the row: `character` and `monster` are pictures owned
-- by a sheet, and `map`, `token`, `avatar` and `music` are library stock owned
-- by the account. Moving the portrait puts it beside `monster`, where it
-- belongs, and leaves the Avatars page as `type = 'avatar'` with no exception
-- clause -- which is the whole point, because an exception clause is a thing to
-- forget in the next query somebody writes.
--
-- The member is appended at the END of the list, which is an instant metadata
-- change: MySQL stores an ENUM by index, so inserting one in the middle would
-- silently renumber every row.
--
-- NOTHING MOVES IN THE BUCKET. A portrait's object stays at
-- users/{owner}/avatars/{id}, and goes on serving, because every read builds
-- its key from assets.file_path rather than from the type -- see serveImage.
-- Only the replace and the rollback ever rebuilt that key, and both now read
-- file_path off the row for exactly this reason.
ALTER TABLE assets
    MODIFY `type` ENUM('map', 'avatar', 'token', 'music', 'journal', 'monster', 'character') NOT NULL DEFAULT 'map';

-- Unconditional, because InsertAvatar was the only statement that ever wrote
-- this member: every `avatar` row in the table is a character's portrait.
UPDATE assets SET `type` = 'character' WHERE `type` = 'avatar';

-- migrate:down
-- THE ROWS GO BEFORE THE ENUM DOES, for the reason 20260906280000 gives:
-- narrowing an ENUM that still holds rows in the member being dropped does not
-- fail, it rewrites them to the empty string.
--
-- THIS MERGES TWO KINDS THAT WENT IN SEPARATELY, and it cannot do otherwise.
-- After the up migration, `avatar` is library stock and `character` is a
-- portrait; this turns both into `avatar` and there is nothing left in the row
-- to tell them apart afterwards. Running it on a database that has collected a
-- library leaves the Avatars page listing portraits again -- which is the
-- hazard the up migration exists to remove.
UPDATE assets SET `type` = 'avatar' WHERE `type` = 'character';

ALTER TABLE assets
    MODIFY `type` ENUM('map', 'avatar', 'token', 'music', 'journal', 'monster') NOT NULL DEFAULT 'map';
