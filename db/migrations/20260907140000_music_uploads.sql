-- migrate:up
-- ONE COLUMN, AND IT EXISTS BECAUSE MUSIC DOES NOT PASS THROUGH THE SERVER.
--
-- Every other upload in this app arrives as a multipart body, is written to R2
-- by the handler that received it, and is finished by the time the response is
-- written. A track is one to two hours long and 115 to 175 MB, so that path
-- would mean a 175 MB allocation, a 175 MB spill to the container's /tmp, and a
-- read deadline amounting to a demand for 2.3 Mbps -- and then the same bytes
-- uploaded a second time, from here to R2. So the browser PUTs them straight to
-- the bucket through a presigned URL, and the server never sees them.
--
-- THAT SPLITS AN UPLOAD INTO TWO REQUESTS WITH A GAP BETWEEN THEM, and this
-- column is what the gap needs. The row is written first, because the row is
-- the ledger for what lives in R2 and the presigned URL has to name a key some
-- row already claims. But between that insert and the browser finishing its
-- PUT -- minutes, on a slow line -- the row names an object that is not there
-- yet, and a card drawn from it would be a player with nothing behind it.
--
-- So: uploaded_at IS NULL means "this row owns a key and nothing else". The
-- listing filters on it, the confirm sets it after checking the object's size
-- and reading its first bytes, and internal/sweep collects the rows that never
-- got one.
--
-- IT IS NULL FOR EVERY OTHER KIND AND THAT IS NOT A BUG. A map, a token, an
-- avatar and a journal image all land inside the request that inserted them, so
-- there is no window for this to describe and nothing reads it for them -- the
-- same way every tile column is NULL for anything that is not a map, and
-- journal_id is NULL for anything that is not a journal image. THE SWEEP BELOW
-- CARRIES type = 'music' IN ITS WHERE FOR EXACTLY THIS REASON: without it, a
-- pass would delete every avatar and every map in the table.
--
-- The index covers the sweep and nothing else: the oldest music rows that never
-- finished uploading.
ALTER TABLE assets
    ADD COLUMN uploaded_at DATETIME NULL AFTER tiled_at,
    ADD INDEX idx_assets_pending_upload (type, uploaded_at, created_at);

-- migrate:down
-- Every pending music row becomes indistinguishable from a finished one, and
-- the listing would then draw a player for an object that may never have
-- landed. There are no rows this can corrupt on the way down that the up
-- migration did not create.
ALTER TABLE assets
    DROP INDEX idx_assets_pending_upload,
    DROP COLUMN uploaded_at;
