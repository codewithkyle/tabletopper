-- migrate:up
ALTER TABLE users
    ADD COLUMN `avatar_asset_id` varbinary(16) DEFAULT NULL AFTER `profile_image_url`;

ALTER TABLE assets
    MODIFY `type` ENUM('map', 'avatar', 'token', 'music', 'journal', 'monster', 'character', 'profile') NOT NULL DEFAULT 'map';

-- migrate:down
UPDATE users SET avatar_asset_id = NULL;
DELETE FROM assets WHERE `type` = 'profile';

ALTER TABLE assets
    MODIFY `type` ENUM('map', 'avatar', 'token', 'music', 'journal', 'monster', 'character') NOT NULL DEFAULT 'map';

ALTER TABLE users
    DROP COLUMN `avatar_asset_id`;
