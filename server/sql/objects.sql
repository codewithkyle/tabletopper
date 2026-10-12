-- name: EnqueueObjectPurge :exec
INSERT INTO object_purge (id, object_key, is_prefix)
VALUES (?, ?, ?);
-- name: ListObjectPurges :many
SELECT id, object_key, is_prefix FROM object_purge
ORDER BY attempts, created_at
LIMIT 100;
-- name: DeleteObjectPurge :exec
DELETE FROM object_purge
WHERE id = ?;
-- name: RecordObjectPurgeAttempt :exec
UPDATE object_purge
SET attempts = attempts + 1
WHERE id = ?;
