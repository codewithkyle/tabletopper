-- name: GetUserByClerkID :one
SELECT id, username, profile_image_url FROM users
WHERE clerk_id = ?;

-- name: CreateUser :exec
INSERT INTO users
(id, username, clerk_id, profile_image_url)
VALUES (?, ?, ?, ?);

-- name: UpdateUserPreferences :exec
UPDATE users
SET theme = sqlc.arg(theme),
    timezone = sqlc.arg(timezone),
    date_format = sqlc.arg(date_format),
    time_format = sqlc.arg(time_format)
WHERE id = sqlc.arg(id);
