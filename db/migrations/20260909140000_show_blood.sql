-- migrate:up
ALTER TABLE users
    ADD COLUMN show_blood BOOLEAN NOT NULL DEFAULT 1 AFTER follow_turn;

-- migrate:down
ALTER TABLE users
    DROP COLUMN show_blood;
