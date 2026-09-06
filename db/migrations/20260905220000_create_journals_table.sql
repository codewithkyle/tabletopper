-- migrate:up
-- Journal entries are markdown, and the markdown lives here rather than in R2.
-- R2 is right for maps and avatars and wrong for this: those are megabytes,
-- served straight to the browser and never queried, while an entry is kilobytes
-- rendered into a page the server is already generating. Storing the body as an
-- object would buy a second write with no transaction around it -- the list
-- needs title and both timestamps, so there is a row either way, and the failure
-- mode is a row whose updated_at moved while the object did not -- plus a
-- network round trip per entry on the list, a billable write on every autosave
-- debounce, and no way to ever run a search. 5,000 entries at 4 KB is 20 MB.
-- inventory.description is the existing precedent.
--
-- Mirrors inventory otherwise: owner_id denormalised so a single-row write
-- filters on id, character_id and owner_id together with no join, and no
-- `position` column because ULIDs sort lexicographically by creation time, so
-- ORDER BY id is creation order and idx_journals_character serves it.
--
-- MEDIUMTEXT, not TEXT. MySQL runs strict, so overflowing 64 KB returns a driver
-- error that reaches the writer as a 500 on a field they were entitled to
-- overfill. The controller caps the body well below this and answers with a
-- message instead -- the same reasoning as the inventory description limit.
--
-- EVERY COLUMN HAS A DEFAULT, body included -- MEDIUMTEXT cannot take a literal
-- default but takes an expression one. That is what lets the insert name three
-- columns and nothing else, so creating an entry cannot carry entry data.
--
-- NO FULLTEXT INDEX. InnoDB maintains one on every UPDATE of the indexed column,
-- and the editor updates body on a one-second debounce for as long as someone is
-- writing. Adding the index to a 20 MB table the day search is built takes
-- seconds; paying for it on every keystroke pause until then does not.
--
-- ON UPDATE CURRENT_TIMESTAMP only fires when a column actually changes, so a
-- debounce that re-posts identical text leaves updated_at alone and the date on
-- the list stays honest.
CREATE TABLE IF NOT EXISTS journals (
    id VARBINARY(16) NOT NULL PRIMARY KEY,
    owner_id VARBINARY(16) NOT NULL,
    character_id VARBINARY(16) NOT NULL,

    title VARCHAR(255) NOT NULL DEFAULT '',
    body MEDIUMTEXT NOT NULL DEFAULT (''),

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    KEY idx_journals_character (character_id, id)
);

-- migrate:down
DROP TABLE IF EXISTS journals;
