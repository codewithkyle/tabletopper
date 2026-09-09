-- migrate:up
-- Whether this account's camera goes to whoever is acting when the turn moves.
--
-- IT IS A FIFTH PREFERENCE COLUMN AND NOT A ROOM SETTING, which is the whole
-- decision in this migration. Everything else about a fight is the table's --
-- who is in the order, what they rolled, whose turn it is -- and this is not:
-- it is about where one person's viewport points. A GM running the fight from
-- an overview and a player who wants to be shown their own turn are looking at
-- the same room, and a room-level switch would make one of them wrong.
--
-- IT IS ALSO NOT localStorage, which is where the window positions live. Those
-- are per browser on purpose; this is a decision somebody makes once about how
-- they like to play, and having to make it again on the laptop is the same
-- complaint the four columns beside it were added to answer.
--
-- BOOLEAN AND NOT AN ENUM, unlike its three neighbours. They are enums because
-- their sets are closed BY DESIGN and adding a member means somebody chose a
-- new rendering, so the database is the right place to refuse anything else.
-- On or off has no third member to add.
--
-- ON BY DEFAULT, AND THAT APPLIES TO EVERY ROW THAT ALREADY EXISTS. A NOT NULL
-- column with a default backfills, so no account has to go and find this to get
-- the behaviour -- and somebody who does not want it turns it off once.
ALTER TABLE users
    ADD COLUMN follow_turn BOOLEAN NOT NULL DEFAULT 1 AFTER time_format;

-- migrate:down
ALTER TABLE users
    DROP COLUMN follow_turn;
