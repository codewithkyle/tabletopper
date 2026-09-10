-- migrate:up
-- Whether this account sees blood on the floor at a table.
--
-- IT IS THE SIXTH PREFERENCE COLUMN AND IT SITS BESIDE follow_turn FOR THE SAME
-- REASON. Neither decides what a page looks like; both decide what one person's
-- canvas does, and both are that person's answer rather than the table's. The
-- GM does not get to decide how much gore is on somebody else's screen, and a
-- player who turns it off has not turned it off for the friend beside them.
--
-- THERE IS NOTHING ON THE SERVER TO SWITCH OFF, which is what makes this a
-- preference and not a room setting. The marks are drawn by each browser out of
-- hit points it watched change; no event carries them and no row records one.
-- So this column does not suppress anything on the wire -- it is read once by
-- the page that renders the tabletop and honoured entirely on the client.
--
-- BOOLEAN AND NOT AN ENUM, and on by default, exactly as follow_turn is. There
-- is no third answer to "do you want to see it", and a NOT NULL column with a
-- default backfills, so every account that already exists keeps the blood it
-- has been playing with and anybody who does not want it turns it off once.
ALTER TABLE users
    ADD COLUMN show_blood BOOLEAN NOT NULL DEFAULT 1 AFTER follow_turn;

-- migrate:down
ALTER TABLE users
    DROP COLUMN show_blood;
