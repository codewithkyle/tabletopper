-- migrate:up
CREATE TABLE saved_rolls (
    id         VARBINARY(16) NOT NULL,
    owner_id   VARBINARY(16) NOT NULL,
    label      VARCHAR(64) NOT NULL DEFAULT '',
    expr       VARCHAR(64) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_saved_rolls_owner (owner_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- migrate:down
DROP TABLE saved_rolls;
