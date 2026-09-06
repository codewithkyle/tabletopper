-- A journal entry belongs to one character and one owner, and every statement
-- below is scoped by all of the ids it has: the entry id and the character id
-- both arrive in the URL and neither is trusted, and the owner comes from the
-- session. A request can name any entry it likes and still only reach its own.
--
-- All three mutations are :execresult, not :exec. The pool runs with found-rows
-- semantics, so zero matched rows means "not this user's", not "nothing
-- changed", and the handler answers 404 rather than reporting a save that never
-- happened.

-- name: ListCharacterJournals :many
-- NEVER `SELECT *`, and never body. The list renders titles and dates; 200
-- entries at 4 KB each is 800 KB read out of the database and thrown away.
--
-- ORDERED BY updated_at, NOT BY id. A journal is a working desk rather than an
-- archive: the entry someone edited last is the one they are still writing, and
-- it belongs at the top even when it was created months before the ones under
-- it. id DESC breaks the tie -- ULIDs sort lexicographically by creation time,
-- so two entries saved in the same second still come out newest first, and the
-- order never wobbles between two renders of the same list.
--
-- THERE IS NO INDEX FOR THIS SORT, deliberately. idx_journals_character finds
-- one character's rows and MySQL sorts them in memory, which for a list of
-- dozens is nothing. An index on (character_id, updated_at) would earn its keep
-- only on a much longer list, and it would be rewritten on every debounced save
-- -- updated_at is the one column that changes on every keystroke pause. That is
-- the same trade the FULLTEXT note in the migration makes, for the same reason.
SELECT id, title, created_at, updated_at FROM journals
WHERE character_id = ? AND owner_id = ?
ORDER BY updated_at DESC, id DESC;

-- name: SearchCharacterJournals :many
-- The search box above the list. Same columns and same order as the list above,
-- so a filtered list and an unfiltered one are the same list.
--
-- NO FULLTEXT INDEX, AND NOT BECAUSE ONE HAS NOT BEEN GOT ROUND TO. Four
-- reasons, and the first is the one that would still hold if the others were
-- fixed:
--
-- A FULLTEXT index is table-wide, but every search here is one character's
-- journal. MATCH in a WHERE clause is what the optimiser plans the read around,
-- so character_id and owner_id stop being the way in and become a filter over
-- what MATCH already found -- the search reads every user's entries and throws
-- away all but one character's. LIKE goes the other way: idx_journals_character
-- narrows to this character's rows first and the bodies scanned are only those.
-- One character's journal is dozens of rows of a few kilobytes. So the cost of a
-- search here grows with how much its own writer has written, and under MATCH it
-- would grow with how much every other writer had.
--
-- The matching would also be wrong for the control. innodb_ft_min_token_size is
-- 3, so a two-character search finds nothing -- and a box that searches as it is
-- typed passes through one and two characters on its way to every longer term.
-- The stopword list drops `the`, `for` and `with`. Boolean mode matches a prefix
-- with `orc*` and never an infix. A box above a list is read as ctrl-F, and
-- LIKE '%term%' is what ctrl-F does.
--
-- The index would be maintained on every UPDATE of body, which is every
-- one-second debounce for as long as someone is writing, which is what the note
-- on the table says it is not worth.
--
-- And the body is markdown, so a token index would be indexing link URLs and
-- heading markers regardless. It would not be matching more cleanly than this,
-- only more expensively.
--
-- LIKE stops being the right answer when one character holds thousands of
-- entries. Should that day come the index is an ALTER on a table of megabytes.
--
-- THE TERM IS A PATTERN, NOT A WORD, and the caller escapes it. `%` and `_` are
-- wildcards to LIKE, both are ordinary characters to a person typing, and an
-- unescaped `%` here matches every entry the character has.
--
-- The table is utf8mb4_0900_ai_ci, so LIKE is already case- and
-- accent-insensitive. Wrapping either side in LOWER() would add nothing and
-- would only make the comparison harder to read.
SELECT id, title, created_at, updated_at FROM journals
WHERE character_id = sqlc.arg(character_id) AND owner_id = sqlc.arg(owner_id)
    AND (title LIKE sqlc.arg(term) OR body LIKE sqlc.arg(term))
ORDER BY updated_at DESC, id DESC;

-- name: GetJournalEntry :one
-- The editor page, and nothing else. This is the one read that carries the body.
SELECT * FROM journals
WHERE id = ? AND character_id = ? AND owner_id = ?;

-- name: InsertJournalEntry :execresult
-- THREE COLUMNS AND NO MORE. title and body both have schema defaults, so a new
-- entry is blank and creating one has nowhere to put entry data -- the same
-- property InsertInventoryItem and CreateCharacterFromName have.
--
-- INSERT ... SELECT rather than VALUES, so the character is the guard. A plain
-- VALUES would take character_id straight from the URL and owner_id from the
-- session, which are consistent with each other but say nothing about whether
-- that character is this user's -- it would hang entries off a stranger's sheet
-- that only the sender could ever see. Selecting owner_id and id off the
-- characters row means a character that is not this user's matches nothing and
-- inserts nothing, and the handler reads that as a 404.
INSERT INTO journals (id, owner_id, character_id)
SELECT sqlc.arg(id), characters.owner_id, characters.id
FROM characters
WHERE characters.id = sqlc.arg(character_id) AND characters.owner_id = sqlc.arg(owner_id);

-- name: UpdateJournalEntry :execresult
UPDATE journals
SET title = ?, body = ?
WHERE id = ? AND character_id = ? AND owner_id = ?;

-- name: DeleteJournalEntry :execresult
DELETE FROM journals
WHERE id = ? AND character_id = ? AND owner_id = ?;
