-- migrate:up
CREATE TABLE IF NOT EXISTS users (
    id  BINARY(16) PRIMARY KEY
);

-- migrate:down
DROP TABLE IF EXISTS users;
