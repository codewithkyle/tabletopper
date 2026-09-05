-- migrate:up
-- Equipment and Resources are gone, and the inventory table replaces both.
--
-- Equipment was already a view of what a character owns wearing a different
-- name -- the column said weapons, the heading said Equipment, and the rows were
-- a name and a description. Resources was the same component again for gear that
-- was not a weapon, which is what an inventory item is. Keeping either would
-- mean two lists that describe the same objects and drift apart, so the Character
-- tab now renders the equipped rows of the inventory table instead and there is
-- one place a thing you own is written down.
--
-- The contents go with the columns. Nothing salvages them into inventory rows
-- because this schema has never been deployed; the data is local and expendable.
ALTER TABLE characters
    DROP COLUMN weapons,
    DROP COLUMN resources;

-- migrate:down
-- The columns come back empty, and with a default the originals did not have --
-- NOT NULL cannot be added to a populated table without one, and JSON takes only
-- an expression default. Down restores the shape, not the contents.
ALTER TABLE characters
    ADD COLUMN weapons JSON NOT NULL DEFAULT ('[]'),
    ADD COLUMN resources JSON NOT NULL DEFAULT ('[]');
