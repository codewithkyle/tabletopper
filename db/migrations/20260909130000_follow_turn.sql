-- migrate:up
ALTER TABLE users
    ADD COLUMN follow_turn BOOLEAN NOT NULL DEFAULT 1 AFTER time_format;

-- migrate:down
ALTER TABLE users
    DROP COLUMN follow_turn;
