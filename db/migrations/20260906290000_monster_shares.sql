-- migrate:up
ALTER TABLE shares
    MODIFY resource_type ENUM('journal', 'character', 'monster') NOT NULL DEFAULT 'journal',
    MODIFY character_id VARBINARY(16) NULL;

-- migrate:down
DELETE FROM shares WHERE resource_type = 'monster';

ALTER TABLE shares
    MODIFY character_id VARBINARY(16) NOT NULL,
    MODIFY resource_type ENUM('journal', 'character') NOT NULL DEFAULT 'journal';
