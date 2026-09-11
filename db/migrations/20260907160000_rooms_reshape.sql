-- migrate:up
DROP TABLE IF EXISTS rooms;

CREATE TABLE rooms (
    id           VARBINARY(16) NOT NULL,
    owner_id     VARBINARY(16) NOT NULL,
    name         VARCHAR(128) NOT NULL,
    code         CHAR(4) CHARACTER SET ascii NULL,
    is_locked    TINYINT(1) NOT NULL DEFAULT 0,
    snapshot     JSON NOT NULL DEFAULT (json_object()),
    snapshot_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,
    snapshot_at  DATETIME NULL,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    closed_at    DATETIME NULL,
    PRIMARY KEY (id),
    UNIQUE KEY ux_rooms_code (code),
    KEY idx_rooms_owner (owner_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE sessions
    ADD INDEX idx_sessions_room_id (room_id);

-- migrate:down
ALTER TABLE sessions
    DROP INDEX idx_sessions_room_id;

DROP TABLE IF EXISTS rooms;

CREATE TABLE IF NOT EXISTS rooms (
    id  VARBINARY(16) PRIMARY KEY NOT NULL,
    owner_id VARBINARY(16) NOT NULL,

    code CHAR(4) CHARACTER SET ascii NOT NULL,
    is_open TINYINT(1) NOT NULL DEFAULT 1,
    is_locked TINYINT(1) NOT NULL DEFAULT 0,

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    closed_at DATETIME NULL,

    UNIQUE KEY ux_rooms_code_open (code, is_open)
);
