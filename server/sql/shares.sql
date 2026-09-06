-- A share is a read grant on one thing, handed out as a link: a journal entry
-- or a whole character sheet. Every statement an owner runs is scoped by the
-- owner and by the thing being shared -- and, for an entry, by the character
-- above it as well; the two statements a visitor runs are scoped by the token
-- alone, because the token IS the authorisation and there is nobody signed in
-- to check it against.
--
-- resource_type IS IN EVERY OWNER STATEMENT and in none of the visitor's. An
-- owner asking about a share already knows which of theirs they mean, so the
-- type is a literal in the WHERE and a sheet's statements can never reach an
-- entry's row; a visitor knows only a token, so the type comes back with the
-- row and SharePage decides what to open.
--
-- REVOKING IS DELETING. There is no revoked_at, no is_active and no second
-- state that can disagree with the first: a link works because its row is
-- there, and stops the moment it is not. It is also what makes the unique key
-- on (resource_type, resource_id) work as "one live link per thing" -- revoke
-- then share again mints a new token, and the old link 404s rather than
-- quietly coming back to life.

-- name: GetJournalShare :one
-- What the share dialog asks before it renders: is this entry already shared,
-- and if so with what expiry and whether a password. It reads the row whether
-- or not it has expired, because an expired share is still a row the owner has
-- to be shown and offered the chance to revoke -- unlike GetShareByToken
-- below, which is a visitor asking to be let in.
SELECT * FROM shares
WHERE resource_type = 'journal' AND resource_id = sqlc.arg(entry_id)
    AND character_id = sqlc.arg(character_id) AND owner_id = sqlc.arg(owner_id);

-- name: InsertJournalShare :execresult
-- INSERT ... SELECT rather than VALUES, and for the reason InsertJournalEntry
-- gives: a VALUES would take the entry id from the URL and the owner from the
-- session, which are consistent with each other and say nothing about whether
-- that entry is this user's. Selecting owner_id and character_id off the
-- journals row means an entry that is not this user's matches nothing, inserts
-- nothing, and is read by the handler as a 404 -- with no window between the
-- check and the write for the entry to be deleted in.
--
-- It is also what makes the two denormalised columns honest. owner_id and
-- character_id are copied off the row they describe rather than off the
-- request, so they cannot disagree with it.
INSERT INTO shares (id, owner_id, character_id, resource_type, resource_id, token, password_hash, expires_at)
SELECT sqlc.arg(id), journals.owner_id, journals.character_id, 'journal', journals.id,
    sqlc.arg(token), sqlc.arg(password_hash), sqlc.arg(expires_at)
FROM journals
WHERE journals.id = sqlc.arg(entry_id) AND journals.character_id = sqlc.arg(character_id)
    AND journals.owner_id = sqlc.arg(owner_id);

-- name: DeleteJournalShare :execresult
-- Revoking one entry's link, and also what deleting the entry runs. The two
-- callers want different things from the result -- the revoke route answers
-- 404 on zero rows, the entry delete does not care -- and that is the whole
-- difference between them, so it is one statement.
DELETE FROM shares
WHERE resource_type = 'journal' AND resource_id = sqlc.arg(entry_id)
    AND character_id = sqlc.arg(character_id) AND owner_id = sqlc.arg(owner_id);

-- The three owner statements for a shared character sheet, which are the three
-- above with one difference: the thing being shared IS the character, so
-- resource_id and character_id hold the same id. Both are still written out,
-- because they are read by different things -- resource_id by the unique key
-- that makes a character's link the only one, character_id by the purge and by
-- the test that reads the schema looking for it.

-- name: GetCharacterShare :one
-- What the sheet's share dialog asks before it renders. Like GetJournalShare it
-- reads the row whether or not it has expired: an expired share is still a row
-- the owner has to be shown and offered the chance to revoke.
SELECT * FROM shares
WHERE resource_type = 'character' AND resource_id = sqlc.arg(character_id)
    AND owner_id = sqlc.arg(owner_id);

-- name: InsertCharacterShare :execresult
-- INSERT ... SELECT for the reason InsertJournalShare gives: taking the ids
-- from the request would prove they are consistent with each other and nothing
-- about whose character this is. Selecting them off the characters row means a
-- character that is not this user's matches nothing, inserts nothing, and is
-- read by the handler as a 404 -- with no window between the check and the
-- write for the character to be deleted in.
--
-- characters.id is written into resource_id and character_id both, from the row
-- rather than from the URL, so the two columns cannot disagree.
INSERT INTO shares (id, owner_id, character_id, resource_type, resource_id, token, password_hash, expires_at)
SELECT sqlc.arg(id), characters.owner_id, characters.id, 'character', characters.id,
    sqlc.arg(token), sqlc.arg(password_hash), sqlc.arg(expires_at)
FROM characters
WHERE characters.id = sqlc.arg(character_id) AND characters.owner_id = sqlc.arg(owner_id);

-- name: DeleteCharacterShare :execresult
-- Revoking the sheet's link, and nothing else: resource_type pins it to the
-- character share, so an owner revoking their sheet keeps every journal link
-- the same character handed out. DeleteSharesForCharacter below is the one that
-- takes them all, and it is deliberately named so the two cannot be reached for
-- by accident.
DELETE FROM shares
WHERE resource_type = 'character' AND resource_id = sqlc.arg(character_id)
    AND owner_id = sqlc.arg(owner_id);

-- name: DeleteSharesForCharacter :exec
-- Deleting a character takes every link it handed out, of every type: its own
-- sheet and each of its entries. It reads character_id directly rather than
-- finding the rows through journals, which is what lets it run in any order
-- beside the other five deletes that request makes -- including after the
-- journals themselves are gone.
DELETE FROM shares
WHERE character_id = ? AND owner_id = ?;

-- name: GetShareByToken :one
-- The visitor's way in, and the only statement in the app whose entire WHERE
-- clause comes from an unauthenticated request. The token is 128 random bits
-- and there is nothing else to check it against, so the row's existence is the
-- authorisation.
--
-- THE EXPIRY IS IN THE STATEMENT, not in the handler and not left to a
-- sweeper. A share stops working the second it expires, whether or not
-- anything has got round to deleting the row -- and a handler that read the
-- row and compared the time itself would be one `if` away from serving an
-- entry the owner believed had stopped being readable.
--
-- It selects no journal columns and no character columns. The password may not
-- have been answered yet, and anything read before that gate would be content
-- the process had in memory with nothing but a later branch keeping it off the
-- wire.
--
-- THE TYPE COMES BACK RATHER THAN GOING IN. This statement used to pin
-- resource_type = 'journal', which made the token name one kind of thing; now
-- the token names a row and the row says what it is, and SharePage is the one
-- place that turns that into a page. A caller that only serves one kind checks
-- the type it got -- see GetShareImage -- rather than asking for a share that
-- is one and reading a miss as a dead link, which would tell a visitor holding
-- a live character link that their link was dead.
SELECT id, owner_id, character_id, resource_type, resource_id, password_hash FROM shares
WHERE token = sqlc.arg(token)
    AND (expires_at IS NULL OR expires_at > NOW());

-- name: GetSharedJournalEntry :one
-- Everything the shared page renders, in one read: the entry, and the handful
-- of character columns the banner above it shows. It is scoped by the entry,
-- the character and the owner -- all three off the share row rather than out
-- of the URL -- so a share whose entry has since been deleted matches nothing
-- and the visitor is told the link is dead.
--
-- NEVER `SELECT *` on either side. characters is fifty columns of sheet and
-- this page shows five of them; a wildcard would read a stranger's whole
-- character sheet into memory to print their name above their diary.
SELECT journals.title, journals.body,
    characters.name, characters.level, characters.classes, characters.race, characters.asset_id
FROM journals
JOIN characters ON characters.id = journals.character_id AND characters.owner_id = journals.owner_id
WHERE journals.id = sqlc.arg(entry_id) AND journals.character_id = sqlc.arg(character_id)
    AND journals.owner_id = sqlc.arg(owner_id);

-- name: DeleteExpiredShares :execresult
-- The sweeper's statement. Nothing depends on it running -- every read above
-- already refuses an expired share -- so this is housekeeping rather than
-- enforcement, and the grace period exists so an owner who opens the dialog
-- the morning after can still see that a link expired rather than that it
-- vanished.
DELETE FROM shares
WHERE expires_at IS NOT NULL AND expires_at < sqlc.arg(cutoff);

-- name: GetSharedCharacterAvatar :one
-- The portrait on the shared page's banner. An INNER JOIN, so a character with
-- no avatar is no row at all and the route answers 404 -- which is what the
-- banner falls back to anyway, since it renders the initial when there is no
-- picture to render.
--
-- updated_at is what the ETag is built from, and it is why this is its own
-- statement rather than GetCharacterAsset with the share row's ids. An avatar
-- is REPLACED at its id: the upload overwrites the same key and bumps the same
-- row, so the id alone would be an ETag that never changed while the bytes
-- behind it did.
SELECT assets.file_path, assets.updated_at
FROM characters
JOIN assets ON assets.id = characters.asset_id
WHERE characters.id = sqlc.arg(character_id) AND characters.owner_id = sqlc.arg(owner_id);
