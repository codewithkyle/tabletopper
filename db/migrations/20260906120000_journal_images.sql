-- migrate:up
-- Journal images are rows in assets like every other object in R2, so there is
-- one ledger of what the bucket holds and one row-first, object-second write
-- path. Two columns are theirs alone and NULL for every other type:
--
-- journal_id ties the image to the entry it was uploaded into. It is what lets
-- the serve route join to journals and check every id in the URL, and what lets
-- a save find the entry's images without scanning the table. It is a soft link:
-- no FOREIGN KEY, because the sweeper has to keep working after the entry is
-- gone.
--
-- detached_at is when the image stopped being referenced by its entry's body,
-- or was uploaded and not yet referenced -- an image is born detached and the
-- first save that carries its URL attaches it. The sweeper deletes the object
-- and the row a day after detachment; until then the image still serves, so an
-- undo lands on an image that is still there. Nothing in a request deletes a
-- journal object.
--
-- The index is on journal_id alone. The per-entry reads are the hot path: one on
-- every debounced save. The sweep runs hourly and scans assets for
-- type = 'journal' AND detached_at < cutoff, which on a table of thousands of
-- rows is nothing, and an index on detached_at would be rewritten on every
-- detach and attach for a query that runs twenty-four times a day.
--
-- Adding an ENUM member at the end is an instant metadata change.
ALTER TABLE assets
    MODIFY COLUMN type ENUM('map', 'avatar', 'token', 'music', 'journal') NOT NULL DEFAULT 'map',
    ADD COLUMN journal_id VARBINARY(16) NULL AFTER owner_id,
    ADD COLUMN detached_at DATETIME NULL AFTER name,
    ADD INDEX idx_assets_journal (journal_id);

-- migrate:down
-- Rows of the new type are dropped, which orphans their objects in R2. A down
-- migration is not a cleanup tool; the objects can be listed and removed under
-- users/*/journals/ by hand.
DELETE FROM assets WHERE type = 'journal';
ALTER TABLE assets
    DROP INDEX idx_assets_journal,
    DROP COLUMN detached_at,
    DROP COLUMN journal_id,
    MODIFY COLUMN type ENUM('map', 'avatar', 'token', 'music') NOT NULL DEFAULT 'map';
