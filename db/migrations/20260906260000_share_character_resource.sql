-- migrate:up
ALTER TABLE shares
    MODIFY resource_type ENUM('journal', 'character') NOT NULL DEFAULT 'journal';

-- migrate:down
DELETE FROM shares WHERE resource_type = 'character';

ALTER TABLE shares
    MODIFY resource_type ENUM('journal') NOT NULL DEFAULT 'journal';
