-- migrate:up
ALTER TABLE assets
    MODIFY `type` ENUM('map', 'avatar', 'token', 'music', 'journal', 'monster', 'character') NOT NULL DEFAULT 'map';

UPDATE assets SET `type` = 'character' WHERE `type` = 'avatar';

-- migrate:down
UPDATE assets SET `type` = 'avatar' WHERE `type` = 'character';

ALTER TABLE assets
    MODIFY `type` ENUM('map', 'avatar', 'token', 'music', 'journal', 'monster') NOT NULL DEFAULT 'map';
