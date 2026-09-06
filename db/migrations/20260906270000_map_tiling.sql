-- migrate:up
-- Ten columns for the tile pyramid a map is served from. Every one of them is
-- NULL for an avatar, a token or a journal image, the way journal_id and
-- detached_at are, because none of those is tiled. tile_attempts is the one
-- exception and the note below says why.
--
-- THE PYRAMID COLUMNS AND THE JOB COLUMNS ARE SEPARATE ON PURPOSE. width,
-- height, tile_size, max_zoom, tile_gen and tiled_at describe the pyramid that
-- is serving right now; tile_state, tile_attempts, tile_lease and
-- tile_leased_at describe the job that is building one. A map whose
-- replacement is being tiled is 'working' while its old pyramid still answers
-- every tile request, so the route that serves tiles reads the first group and
-- never looks at the second. One state column doing both jobs would mean a
-- replacement blanks the map for as long as the rebuild takes.
--
-- A GENERATION IS A WHOLE PYRAMID UNDER ITS OWN PREFIX, AND IS NEVER MODIFIED.
-- tile_gen and tile_lease both name one: tile_gen is the generation that is
-- serving, and is in the URL of every tile it holds; tile_lease is the one
-- being built. tile_lease doubles as the token that claims the row, because
-- MySQL has no RETURNING and a conditional UPDATE followed by a SELECT on the
-- token is how a worker finds the row it just took -- and because one value
-- naming both the claim and the prefix it writes is what makes an abandoned
-- job collectable. A 'working' row whose tile_leased_at has gone stale names
-- exactly the prefix to delete before it is queued again.
--
-- width AND height ARE THE ORIENTED PIXELS, not the dimensions in the file's
-- header. A phone photo carrying an EXIF rotation tag decodes transposed, and
-- the tile grid is computed from these two numbers, so they are written by
-- whatever decoded the image rather than by the request that received it --
-- in the same statement as tile_gen, so the row's description of the pyramid
-- and the pyramid itself change together. tile_size is stored rather than
-- assumed so it can change without a re-upload: a different tile size is a new
-- generation built from the retained original.
--
-- tile_attempts is NOT NULL DEFAULT 0 where the rest are nullable, because it
-- is counted rather than read. `tile_attempts = tile_attempts + 1` against a
-- NULL yields NULL, so a poison upload would never accumulate a count and
-- never stop being retried. Zero attempts is a true statement about a row that
-- is not a map, which is why the default costs nothing.
--
-- THE COLUMN IS tile_state, NOT tile_status. Tailwind reads .templ files as
-- text and takes a class-name candidate from any bare lowercase word it finds,
-- `status` is a DaisyUI component, and a single occurrence emits its whole
-- family into the built stylesheet. Column names become sqlc identifiers and
-- sqlc identifiers reach those files. `state` cannot do it.
--
-- The index covers the claim query and nothing else does: the oldest pending
-- map, once per tick.
ALTER TABLE assets
    ADD COLUMN width INT UNSIGNED NULL AFTER detached_at,
    ADD COLUMN height INT UNSIGNED NULL AFTER width,
    ADD COLUMN tile_size SMALLINT UNSIGNED NULL AFTER height,
    ADD COLUMN max_zoom TINYINT UNSIGNED NULL AFTER tile_size,
    ADD COLUMN tile_gen VARBINARY(16) NULL AFTER max_zoom,
    ADD COLUMN tile_state ENUM('pending', 'working', 'ready', 'failed') NULL AFTER tile_gen,
    ADD COLUMN tile_attempts TINYINT UNSIGNED NOT NULL DEFAULT 0 AFTER tile_state,
    ADD COLUMN tile_lease VARBINARY(16) NULL AFTER tile_attempts,
    ADD COLUMN tile_leased_at DATETIME NULL AFTER tile_lease,
    ADD COLUMN tiled_at DATETIME NULL AFTER tile_leased_at,
    ADD INDEX idx_assets_tile_state (tile_state, created_at);

-- migrate:down
-- Every tile in the bucket is orphaned by this, and unreachably so: a tile's
-- key is built from tile_gen and there is nowhere else the generation is
-- written down. A down migration is not a cleanup tool; the objects can be
-- listed and removed under users/*/maps/ by hand.
ALTER TABLE assets
    DROP INDEX idx_assets_tile_state,
    DROP COLUMN tiled_at,
    DROP COLUMN tile_leased_at,
    DROP COLUMN tile_lease,
    DROP COLUMN tile_attempts,
    DROP COLUMN tile_state,
    DROP COLUMN tile_gen,
    DROP COLUMN max_zoom,
    DROP COLUMN tile_size,
    DROP COLUMN height,
    DROP COLUMN width;
