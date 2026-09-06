-- migrate:up
-- Four columns on users rather than a user_settings table. Each is one scalar,
-- each is 1:1 with the user, and every one has a default that is correct for a
-- row nobody has touched -- so there is no backfill, no NULL to handle, and no
-- join to write on the read path.
--
-- THEY ARE READ THROUGH THE SESSION QUERY, joined rather than denormalised into
-- sessions. username and profile_image_url on that table are copies because
-- they only change when a login refreshes them out of Clerk; these change while
-- the user is sitting in the app, and one user has several sessions. A copy
-- would mean changing the theme on a laptop and watching the phone keep the old
-- one until its session expired.
--
-- theme, date_format and time_format ARE ENUMS AND timezone IS NOT, and the
-- line between them is whether the set is closed by design. The three enums are
-- decisions -- adding a member means someone chose a new rendering -- so the
-- database is the right place to refuse anything else. Zone names are a curated
-- convenience over a list of several hundred that grows whenever a reader asks
-- for one, and the real validator is the Go map that also resolves them.
--
-- WHAT IS STORED IS THE INTENT, NEVER THE RENDERING. theme holds light/dark and
-- not caramellatte/coffee: those are DaisyUI theme names, this app has already
-- renamed one of them once, and a rename would have been a migration. Likewise
-- date_format holds dmy_slash and not "02/01/2006" -- a Go layout string is an
-- implementation detail of one package, and a column holding one cannot be
-- checked by anything.
--
-- Appending a member to an ENUM is an instant metadata change; reordering one
-- is not, because the values are stored as their index. So new formats go on
-- the end.
--
-- ascii_bin on timezone for the reason shares.token has it: IANA names are
-- case-sensitive, and under the table's default collation America/New_York and
-- america/new_york would compare equal.
ALTER TABLE users
    ADD COLUMN theme ENUM('system', 'light', 'dark') NOT NULL DEFAULT 'system',
    ADD COLUMN timezone VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'America/New_York',
    ADD COLUMN date_format ENUM('dmy_text', 'mdy_text', 'mdy_slash', 'dmy_slash', 'iso') NOT NULL DEFAULT 'dmy_text',
    ADD COLUMN time_format ENUM('12h', '24h') NOT NULL DEFAULT '12h';

-- migrate:down
ALTER TABLE users
    DROP COLUMN time_format,
    DROP COLUMN date_format,
    DROP COLUMN timezone,
    DROP COLUMN theme;
