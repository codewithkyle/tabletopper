-- migrate:up
CREATE TABLE object_purge (
    id VARBINARY(16) NOT NULL,
    object_key VARCHAR(1024) NOT NULL,
    is_prefix TINYINT(1) NOT NULL DEFAULT 0,
    attempts INT UNSIGNED NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_object_purge_order (attempts, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- migrate:down
DROP TABLE object_purge;
