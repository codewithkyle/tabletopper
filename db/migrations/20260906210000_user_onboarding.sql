-- migrate:up
ALTER TABLE users
    ADD COLUMN onboarded_at DATETIME NULL;

UPDATE users SET onboarded_at = NOW() WHERE onboarded_at IS NULL;

-- migrate:down
ALTER TABLE users
    DROP COLUMN onboarded_at;
