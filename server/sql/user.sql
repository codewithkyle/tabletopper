-- name: GetUserByClerkID :one
SELECT id, username, profile_image_url FROM users
WHERE clerk_id = ?;

-- name: GetUserAvatar :one
SELECT u.avatar_asset_id, a.file_path FROM users u
LEFT JOIN assets a ON a.id = u.avatar_asset_id AND a.owner_id = u.id
WHERE u.id = ?;

-- name: SetUserAvatar :exec
UPDATE users
SET avatar_asset_id = sqlc.arg(avatar_asset_id)
WHERE id = sqlc.arg(id);

-- name: CreateUser :exec
INSERT INTO users
(id, username, clerk_id, profile_image_url)
VALUES (?, ?, ?, ?);

-- name: SetUsername :exec
UPDATE users
SET username = sqlc.arg(username)
WHERE id = sqlc.arg(id);

-- name: UpdateUserPreferences :exec
UPDATE users
SET username = sqlc.arg(username),
    theme = sqlc.arg(theme),
    timezone = sqlc.arg(timezone),
    date_format = sqlc.arg(date_format),
    time_format = sqlc.arg(time_format),
    follow_turn = sqlc.arg(follow_turn),
    show_blood = sqlc.arg(show_blood),
    ping_volume = sqlc.arg(ping_volume)
WHERE id = sqlc.arg(id);

-- name: CompleteOnboarding :exec
UPDATE users
SET username = sqlc.arg(username),
    theme = sqlc.arg(theme),
    timezone = sqlc.arg(timezone),
    date_format = sqlc.arg(date_format),
    time_format = sqlc.arg(time_format),
    follow_turn = sqlc.arg(follow_turn),
    show_blood = sqlc.arg(show_blood),
    ping_volume = sqlc.arg(ping_volume),
    onboarded_at = COALESCE(onboarded_at, NOW())
WHERE id = sqlc.arg(id);

-- name: DismissOnboarding :exec
UPDATE users
SET onboarded_at = COALESCE(onboarded_at, NOW())
WHERE id = sqlc.arg(id);
