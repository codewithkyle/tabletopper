-- migrate:up
-- THE DISPLAY NAME BECOMES THE ACCOUNT'S OWN, so the copy of it on every
-- session row has to go.
--
-- users.username has always been what Clerk said at sign-up, and sessions
-- carried a copy because that value only ever changed when a login refreshed
-- it. Both halves of that are now false: an account can rename itself from the
-- settings dialog, and one account has several sessions -- so a copy would mean
-- renaming yourself on a laptop and watching the phone show the old name until
-- its session expired a week later. It is exactly the reasoning that put the
-- four preference columns on a join instead of on this row.
--
-- GetSession already INNER JOINs users on its primary key, so reading the name
-- off that join costs nothing that was not already being paid.
--
-- profile_image_url stays where it is, and the asymmetry is deliberate: the
-- avatar comes from Clerk, this app has no way to change it, and a login is
-- still the only thing that can.
ALTER TABLE sessions
    DROP COLUMN username;

-- migrate:down
ALTER TABLE sessions
    ADD COLUMN username VARCHAR(128) NOT NULL DEFAULT '' AFTER room_id;

UPDATE sessions s
INNER JOIN users u ON u.id = s.user_id
SET s.username = u.username;
