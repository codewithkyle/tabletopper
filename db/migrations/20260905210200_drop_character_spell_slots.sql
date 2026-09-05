-- migrate:up
-- The last of the spell JSON. spell_slots held all ten levels as one object --
-- counters and the spells themselves -- so every save rewrote all ten and a
-- spell had no identity beyond its position in an array.
--
-- The spells table holds the spells now and the spell_slots table holds the
-- counters, which is what lets a spell be referenced from somewhere that is not
-- the list it was typed into. Nothing salvages the column into rows: this schema
-- has never been deployed and the data is local and expendable.
ALTER TABLE characters DROP COLUMN spell_slots;

-- migrate:down
-- Back empty, and with a default the original did not have -- NOT NULL cannot be
-- added to a populated table without one, and JSON takes only an expression
-- default. Down restores the shape, not the contents.
ALTER TABLE characters ADD COLUMN spell_slots JSON NOT NULL DEFAULT ('{}');
