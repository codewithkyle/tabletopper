-- migrate:up
ALTER TABLE users
    ADD COLUMN deleted_at DATETIME NULL AFTER onboarded_at,
    ADD INDEX idx_users_deleted (deleted_at);

-- migrate:down
ALTER TABLE users
    DROP INDEX idx_users_deleted,
    DROP COLUMN deleted_at;
