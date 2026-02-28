-- name: GetUserByClerkID :one
SELECT id, username, profile_image_url FROM users
WHERE clerk_id = ?;

-- name: CreateUserWithAvatar :exec
INSERT INTO users
(id, username, clerk_id, profile_image_url)
VALUES (?, ?, ?, ?);

-- name: CreateUser :exec
INSERT INTO users
(id, username, clerk_id)
VALUES (?, ?, ?);
