-- name: GetUserByClerkID :one
SELECT id, username, profile_image_url FROM users
WHERE clerk_id = ?;

-- GetUserAvatar is what the upload handler asks before it writes: the asset this
-- account's picture is stored in, and where that asset's bytes live.
--
-- THE LEFT JOIN IS WHY BOTH COLUMNS COME BACK. users.avatar_asset_id has no
-- foreign key behind it -- nothing in this schema does -- so it can name a row
-- that has been deleted, and the join hands that back as an id with no
-- file_path. The handler branches on the PATH and not on the id, which turns a
-- dangling pointer into a fresh upload rather than a write to an empty key.
--
-- The join is scoped to the owner as well as the id, so a pointer that somehow
-- named somebody else's asset reads as no picture rather than as a key this
-- account is about to overwrite.
-- name: GetUserAvatar :one
SELECT u.avatar_asset_id, a.file_path FROM users u
LEFT JOIN assets a ON a.id = u.avatar_asset_id AND a.owner_id = u.id
WHERE u.id = ?;

-- SetUserAvatar points the account at the asset holding its picture. It is only
-- ever called with a freshly inserted id: a REPLACEMENT overwrites the object
-- the row already names and leaves the pointer alone.
-- name: SetUserAvatar :exec
UPDATE users
SET avatar_asset_id = sqlc.arg(avatar_asset_id)
WHERE id = sqlc.arg(id);

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
    time_format = sqlc.arg(time_format),
    follow_turn = sqlc.arg(follow_turn),
    show_blood = sqlc.arg(show_blood)
WHERE id = sqlc.arg(id);

-- name: CompleteOnboarding :exec
-- The welcome dialog answered. It writes the same seven columns the settings
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
    follow_turn = sqlc.arg(follow_turn),
    show_blood = sqlc.arg(show_blood),
    onboarded_at = COALESCE(onboarded_at, NOW())
WHERE id = sqlc.arg(id);

-- name: DismissOnboarding :exec
-- "Not now": the account is marked set up and keeps every default. It writes no
-- setting at all, so it cannot store whatever happened to be sitting in the
-- pickers when the button was pressed.
UPDATE users
SET onboarded_at = COALESCE(onboarded_at, NOW())
WHERE id = sqlc.arg(id);
