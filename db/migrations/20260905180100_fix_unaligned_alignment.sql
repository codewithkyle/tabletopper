-- migrate:up
-- The "Unaligned" option was stored as the misspelling 'unaliged', and the
-- form kept emitting it because it was the value in every existing row.
UPDATE characters SET alignment = 'unaligned' WHERE alignment = 'unaliged';

-- migrate:down
UPDATE characters SET alignment = 'unaliged' WHERE alignment = 'unaligned';
