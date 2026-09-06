-- migrate:up
-- Attacks are one entity per attack, the way inventory and spells are, and for
-- the same two reasons. A row carries six fields, and the JSON repeaters zip
-- parallel slices of form values in document order -- a shape that holds for the
-- two features carries but strains at six. And the whole list is rewritten on
-- every debounce, so typing a longsword's damage would rewrite the other four
-- attacks and churn their identities with it.
--
-- WHAT A CHARACTER CAN DO ON THEIR TURN HAD NOWHERE TO LIVE UNTIL NOW. A monster
-- in this app has actions, bonus_actions, reactions and legendary_actions; a
-- player character had a features list and an inventory description. This is the
-- table that closes that gap, and it is the one part of the sheet a player reads
-- every single round.
--
-- attack_bonus IS TEXT, and it is text because the sheet's own column is
-- "ATK/DC": a longsword puts +7 there and a save cantrip puts DC 15, and both
-- are the answer to "what do I roll or what do they roll". A number could hold
-- one of them.
--
-- damage_type and mastery are VARCHAR with a Go allowlist rather than ENUMs,
-- which is how spells.school already handles the same situation. All three sets
-- are closed by the rules, and the reason none of them is an ENUM is that the
-- validator has to run in Go anyway to normalise what a select posts -- so an
-- ENUM would be a second copy of the list that fails as a 500 instead of a
-- correction. mastery is the 2024 weapon property (Vex, Topple, Sap and the
-- rest) and is empty for most rows: an unarmed strike has none, a spell has
-- none, and a character without the Weapon Mastery feature has none at all.
--
-- owner_id is denormalised the way inventory and spells do it, so a single-row
-- write filters on id, character_id and owner_id together with no join. All
-- three, because the attack id and the character id both arrive in the URL and
-- neither is trusted.
--
-- No `position` column, for inventory's reason: ULIDs sort lexicographically by
-- creation time, so ORDER BY id is insertion order and idx_attacks_character
-- serves it. Alphabetical would slide a freshly added blank row around the list
-- as its name was typed.
--
-- EVERY COLUMN HAS A DEFAULT, notes included -- TEXT cannot take a literal
-- default but takes an expression one. That is what lets the insert name three
-- columns and nothing else, so adding an attack cannot carry attack data.
CREATE TABLE IF NOT EXISTS attacks (
    id VARBINARY(16) NOT NULL PRIMARY KEY,
    owner_id VARBINARY(16) NOT NULL,
    character_id VARBINARY(16) NOT NULL,

    name VARCHAR(128) NOT NULL DEFAULT '',
    attack_bonus VARCHAR(32) NOT NULL DEFAULT '',
    damage VARCHAR(64) NOT NULL DEFAULT '',
    damage_type VARCHAR(32) NOT NULL DEFAULT '',
    mastery VARCHAR(32) NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT (''),

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    KEY idx_attacks_character (character_id, id)
);

-- migrate:down
DROP TABLE IF EXISTS attacks;
