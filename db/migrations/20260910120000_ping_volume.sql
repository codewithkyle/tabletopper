-- migrate:up
-- How loud a ping is on this account's tabletop, as a percentage of the sound's
-- own level.
--
-- IT IS THE SEVENTH PREFERENCE COLUMN AND THE THIRD THAT IS ABOUT A CANVAS
-- rather than a page, beside follow_turn and show_blood. All three decide what
-- one person's table does and none of them is the room's: a GM does not get to
-- decide how much noise comes out of somebody else's laptop, and a player who
-- turns it down has not turned it down for the friend beside them.
--
-- THERE IS NOTHING ON THE SERVER TO TURN DOWN, which is what makes this a
-- preference and not a room setting. A ping is one transient event carrying a
-- point and a person; the noise is synthesised in the browser that receives it,
-- so no event and no row knows a sound happened. This column is read once by the
-- page that renders the tabletop and honoured entirely on the client.
--
-- A PERCENTAGE AND NOT A BOOLEAN, which is the one way it differs from its two
-- neighbours. "Do you want to see blood" has two answers; "how loud" has as
-- many as somebody's room needs -- headphones, a voice call, a quiet house --
-- and the bottom of the range is the mute, so the switch is inside the dial
-- rather than beside it.
--
-- TINYINT UNSIGNED holds 0 to 255 in one byte and the value is 0 to 100, which
-- leaves the check to the one place that can explain a refusal. Anything outside
-- the range is clamped on read rather than rejected: this is a slider position,
-- and a page that would not load because a byte was 120 is worse than a page
-- that plays at full volume.
--
-- DEFAULT 100 SO NOTHING CHANGES FOR ANYBODY. A NOT NULL column with a default
-- backfills, so every account that already exists keeps the level it has been
-- playing at, and somebody who wants it quieter says so once.
ALTER TABLE users
    ADD COLUMN ping_volume TINYINT UNSIGNED NOT NULL DEFAULT 100 AFTER show_blood;

-- migrate:down
ALTER TABLE users
    DROP COLUMN ping_volume;
