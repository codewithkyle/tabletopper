-- migrate:up
-- Whether this account has ever been asked to set itself up. NULL means it has
-- not, and the welcome dialog opens on the homepage until something answers.
--
-- IT IS A COLUMN AND NOT A URL FRAGMENT. The obvious cheap version of this is
-- redirecting a new signup to /#welcome and opening the dialog when the hash is
-- there, and it fails in the one case the feature exists for: a fragment is
-- never sent to the server, so it needs a script to read, and it is gone the
-- moment the browser navigates. Someone who presses Escape in the first ten
-- seconds of their account -- which is when people dismiss everything -- is
-- never asked again, and reads New York dates forever.
--
-- IT CANNOT BE DERIVED FROM THE SETTINGS THEMSELVES. A row holding system,
-- America/New_York, dmy_text and 12h is either an account nobody has set up or
-- an account that deliberately chose exactly the defaults, and treating those
-- two the same would keep asking the second one.
--
-- A TIMESTAMP RATHER THAN A FLAG, for the same price. It answers "has this
-- account been asked" by being NULL or not, and "when" for free -- and the
-- writes use COALESCE so the first answer's time is the one that survives a
-- later save.
--
-- Existing rows are backfilled to the migration time rather than left NULL:
-- they belong to people already using the app, and a welcome dialog is not
-- something to greet them with. New rows take the column default, which is
-- NULL.
ALTER TABLE users
    ADD COLUMN onboarded_at DATETIME NULL;

UPDATE users SET onboarded_at = NOW() WHERE onboarded_at IS NULL;

-- migrate:down
ALTER TABLE users
    DROP COLUMN onboarded_at;
