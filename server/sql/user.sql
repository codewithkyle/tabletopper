-- name: GetUserByClerkID :one
SELECT id, username, profile_image_url FROM users
WHERE clerk_id = ?;

-- name: CreateUser :exec
INSERT INTO users
(id, username, clerk_id, profile_image_url)
VALUES (?, ?, ?, ?);

-- SetUsername seeds a display name onto an account that has none.
--
-- IT IS NOT A LOGIN REFRESH. users.username is the account's own display name:
-- Clerk supplies the first one at sign-up and never overwrites it again, or the
-- rename in the settings dialog would be undone by the next login. This is for
-- the one row that shape leaves behind -- an account created before there was a
-- fallback, whose provider gave no username at all, and which has been carrying
-- an empty name ever since.
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
    time_format = sqlc.arg(time_format)
WHERE id = sqlc.arg(id);

-- name: CompleteOnboarding :exec
-- The welcome dialog answered. It writes the same five columns the settings
-- dialog does and stamps the account as set up in one statement, so there is no
-- window where the settings took and the stamp did not -- which would reopen
-- the dialog over the answer that had just been given.
--
-- COALESCE keeps the first stamp. Nothing reaches this twice today, but the
-- column means "when was this account first set up" and a re-run should not
-- rewrite that.
UPDATE users
SET username = sqlc.arg(username),
    theme = sqlc.arg(theme),
    timezone = sqlc.arg(timezone),
    date_format = sqlc.arg(date_format),
    time_format = sqlc.arg(time_format),
    onboarded_at = COALESCE(onboarded_at, NOW())
WHERE id = sqlc.arg(id);

-- name: DismissOnboarding :exec
-- "Not now": the account is marked set up and keeps every default. It writes no
-- setting at all, so it cannot store whatever happened to be sitting in the
-- pickers when the button was pressed.
UPDATE users
SET onboarded_at = COALESCE(onboarded_at, NOW())
WHERE id = sqlc.arg(id);
