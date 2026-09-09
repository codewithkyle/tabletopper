-- migrate:up
-- AN ACCOUNT GETS A PICTURE OF ITS OWN, AND CLERK'S IS KEPT BESIDE IT.
--
-- users.profile_image_url is what Clerk said at the last login, or the shared
-- placeholder when Clerk had nothing. It stays exactly as it is. What arrives
-- here is a SECOND source that wins over it, so uploading a picture is an
-- override rather than a destructive replace: the Clerk copy goes on being
-- refreshed by every login, and an account that later clears its upload falls
-- back to whatever Clerk has now rather than to whatever Clerk had then.
--
-- IT IS AN ASSET ROW AND NOT A URL COLUMN, which is what buys the replace path
-- for free -- the second upload overwrites the same key, so nothing is ever
-- orphaned and the sweeper has nothing to collect. It is nullable because most
-- accounts will never set one.
--
-- NO FOREIGN KEY, because nothing in this schema has one. A row pointing at an
-- asset that is gone reads as an account with no upload, which is the same
-- thing every other asset pointer here does.
ALTER TABLE users
    ADD COLUMN `avatar_asset_id` varbinary(16) DEFAULT NULL AFTER `profile_image_url`;

-- `profile` IS ITS OWN MEMBER AND NOT `avatar`, for the reason 20260907120000
-- gave when it moved character portraits out of that member: the axis these
-- divide on is what OWNS the row. `map`, `avatar`, `token` and `music` are
-- library stock the asset manager lists and offers a Delete on; `character`,
-- `monster` and now `profile` are pictures owned by something else.
--
-- Putting an account's own picture in `avatar` would list it on the Avatars
-- page, where it would be spawnable as an NPC's face and deletable in one
-- click -- and deleting it would leave users.avatar_asset_id pointing at
-- nothing, which renders as a broken picture and logs nothing at all.
--
-- The member is appended at the END, which is an instant metadata change:
-- MySQL stores an ENUM by index, so inserting one in the middle would silently
-- renumber every row.
ALTER TABLE assets
    MODIFY `type` ENUM('map', 'avatar', 'token', 'music', 'journal', 'monster', 'character', 'profile') NOT NULL DEFAULT 'map';

-- migrate:down
UPDATE users SET avatar_asset_id = NULL;
DELETE FROM assets WHERE `type` = 'profile';

ALTER TABLE assets
    MODIFY `type` ENUM('map', 'avatar', 'token', 'music', 'journal', 'monster', 'character') NOT NULL DEFAULT 'map';

ALTER TABLE users
    DROP COLUMN `avatar_asset_id`;
