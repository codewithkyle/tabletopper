-- migrate:up
CREATE TABLE scenes (
    id           VARBINARY(16) NOT NULL,
    owner_id     VARBINARY(16) NOT NULL,
    name         VARCHAR(128) NOT NULL,
    body         JSON NOT NULL,
    keep_changes TINYINT(1) NOT NULL DEFAULT 0,
    preview_id   VARBINARY(16) NULL,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_scenes_owner (owner_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE rooms
    ADD COLUMN scene_id VARBINARY(16) NULL AFTER snapshot_failed_at;

-- migrate:down
ALTER TABLE rooms
    DROP COLUMN scene_id;

DROP TABLE scenes;
