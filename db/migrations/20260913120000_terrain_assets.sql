-- migrate:up
ALTER TABLE assets
    MODIFY `type` ENUM('map', 'avatar', 'token', 'music', 'journal', 'monster', 'character', 'profile', 'terrain') NOT NULL DEFAULT 'map';

-- migrate:down
DELETE FROM assets WHERE `type` = 'terrain';

ALTER TABLE assets
    MODIFY `type` ENUM('map', 'avatar', 'token', 'music', 'journal', 'monster', 'character', 'profile') NOT NULL DEFAULT 'map';
