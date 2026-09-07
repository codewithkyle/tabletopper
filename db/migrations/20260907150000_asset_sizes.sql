-- migrate:up
-- WHAT AN ACCOUNT IS COSTING, WHICH NOTHING IN THIS SCHEMA COULD ANSWER. Every
-- limit in the app is per-request or per-row -- a map's 128 MiB, a track's
-- 256 MB, forty images in a journal entry -- and none of them is per-owner, so
-- an account's footprint was a question that could only be answered by listing
-- the bucket. This column is the answer kept where the rest of the ledger is.
--
-- THE ROW IS ALREADY THE LEDGER FOR WHAT LIVES IN R2, and this is one more fact
-- about the object it names rather than a new responsibility. Every upload path
-- knows the number for nothing: a map has its multipart part's declared size, a
-- portrait, token, avatar, monster picture and journal image are each a WebP
-- this process encoded and can measure, and a track is the ContentLength the
-- confirm's HEAD already reads back off the bucket.
--
-- ZERO AND NOT NULL, so SUM never has to be wrapped and no reader has to decide
-- what an unknown size means. Rows written before this migration keep the
-- default and are deliberately not backfilled: knowing their sizes would take a
-- HEAD per object, and the total is a running figure for an operator rather
-- than an accounting record. It reads low until those rows are replaced, which
-- is not worth a caveat on the page -- this has only ever run against a local
-- database, where those rows are a handful of the developer's own.
--
-- BIGINT rather than INT: a 256 MB track is comfortably inside a signed 32-bit
-- integer, but a column that has to be widened later is a column that gets
-- widened during an incident.
--
-- NO QUOTA IS ENFORCED. There is deliberately no cap here and no cap in the
-- handlers; what this buys is that adding one later is a constant and a SUM,
-- account-wide across every type, rather than a migration and a backfill in the
-- middle of needing it.
ALTER TABLE assets
    ADD COLUMN size_bytes BIGINT NOT NULL DEFAULT 0 AFTER file_name;

-- migrate:down
ALTER TABLE assets
    DROP COLUMN size_bytes;
