-- migrate:up
ALTER TABLE sessions
    DROP COLUMN username;

-- migrate:down
ALTER TABLE sessions
    ADD COLUMN username VARCHAR(128) NOT NULL DEFAULT '' AFTER room_id;

UPDATE sessions s
INNER JOIN users u ON u.id = s.user_id
SET s.username = u.username;
