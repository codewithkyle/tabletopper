-- name: ListCharacterJournals :many
SELECT id, title, created_at, updated_at FROM journals
WHERE character_id = ? AND owner_id = ?
ORDER BY updated_at DESC, id DESC;

-- name: SearchCharacterJournals :many
SELECT id, title, body, created_at, updated_at FROM journals
WHERE character_id = sqlc.arg(character_id) AND owner_id = sqlc.arg(owner_id)
    AND (title LIKE sqlc.arg(term) OR body LIKE sqlc.arg(term))
ORDER BY updated_at DESC, id DESC;

-- name: GetJournalEntry :one
SELECT * FROM journals
WHERE id = ? AND character_id = ? AND owner_id = ?;

-- name: InsertJournalEntry :execresult
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

-- name: CountJournalImages :one
SELECT COUNT(a.id) AS images
FROM journals j
LEFT JOIN assets a ON a.journal_id = j.id AND a.type = 'journal'
WHERE j.id = ? AND j.character_id = ? AND j.owner_id = ?
GROUP BY j.id;

-- name: InsertJournalImage :exec
INSERT INTO assets (id, owner_id, journal_id, file_path, type, file_name, name, size_bytes, detached_at)
VALUES (?, ?, ?, ?, 'journal', ?, ?, ?, NOW());

-- name: GetJournalImage :one
SELECT a.file_path
FROM assets a
JOIN journals j ON j.id = a.journal_id
WHERE a.id = sqlc.arg(asset_id) AND a.type = 'journal'
    AND j.id = sqlc.arg(entry_id) AND j.character_id = sqlc.arg(character_id)
    AND j.owner_id = sqlc.arg(owner_id);

-- name: ListJournalImageStates :many
SELECT id, detached_at FROM assets
WHERE journal_id = ? AND owner_id = ? AND type = 'journal';

-- name: AttachJournalImage :exec
UPDATE assets SET detached_at = NULL
WHERE id = ? AND owner_id = ? AND type = 'journal';

-- name: DetachJournalImage :exec
UPDATE assets SET detached_at = NOW()
WHERE id = ? AND owner_id = ? AND type = 'journal' AND detached_at IS NULL;

-- name: DetachJournalImages :exec
UPDATE assets SET detached_at = NOW()
WHERE journal_id = ? AND owner_id = ? AND type = 'journal' AND detached_at IS NULL;

-- name: ListCharacterJournalImages :many
SELECT assets.file_path
FROM assets
JOIN journals ON journals.id = assets.journal_id
WHERE journals.character_id = sqlc.arg(character_id) AND journals.owner_id = sqlc.arg(owner_id)
    AND assets.type = 'journal';

-- name: DeleteCharacterJournalImages :exec
DELETE FROM assets
WHERE assets.owner_id = sqlc.arg(owner_id) AND assets.type = 'journal'
    AND assets.journal_id IN (
        SELECT journals.id FROM journals
        WHERE journals.character_id = sqlc.arg(character_id) AND journals.owner_id = sqlc.arg(owner_id)
    );

-- name: DeleteCharacterJournals :exec
DELETE FROM journals
WHERE character_id = ? AND owner_id = ?;

-- name: ListSweepableJournalImages :many
SELECT id, file_path FROM assets
WHERE type = 'journal' AND detached_at IS NOT NULL AND detached_at < sqlc.arg(cutoff)
ORDER BY detached_at
LIMIT 100;

-- name: DeleteSweptJournalImage :execresult
DELETE FROM assets
WHERE id = ? AND type = 'journal' AND detached_at IS NOT NULL AND detached_at < sqlc.arg(cutoff);
