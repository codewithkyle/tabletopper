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
