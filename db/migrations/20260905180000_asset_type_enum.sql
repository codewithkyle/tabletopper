-- migrate:up
-- assets.type was a TINYINT with the meaning kept in a comment (0 map,
-- 1 avatar, 2 token, 3 music) and repeated as magic numbers in every query.
-- As an ENUM the values are the names, and sqlc generates typed constants
-- for them (AssetsTypeMap, AssetsTypeAvatar, ...).
--
-- Converted through a second column rather than MODIFY: MySQL casts a number
-- to an ENUM by its 1-based index, so 0 would become '' and everything else
-- would shift by one.
ALTER TABLE assets
    DROP CHECK chk_assets_type,
    DROP INDEX idx_assets_owner_type,
    ADD COLUMN kind ENUM('map', 'avatar', 'token', 'music') NOT NULL DEFAULT 'map' AFTER type;

UPDATE assets SET kind = CASE type
    WHEN 0 THEN 'map'
    WHEN 1 THEN 'avatar'
    WHEN 2 THEN 'token'
    ELSE 'music'
END;

ALTER TABLE assets
    DROP COLUMN type,
    RENAME COLUMN kind TO type,
    ADD INDEX idx_assets_owner_type (owner_id, type);

-- migrate:down
ALTER TABLE assets
    DROP INDEX idx_assets_owner_type,
    ADD COLUMN kind TINYINT UNSIGNED NOT NULL DEFAULT 0 AFTER type;

UPDATE assets SET kind = CASE type
    WHEN 'map' THEN 0
    WHEN 'avatar' THEN 1
    WHEN 'token' THEN 2
    ELSE 3
END;

ALTER TABLE assets
    DROP COLUMN type,
    RENAME COLUMN kind TO type,
    ADD INDEX idx_assets_owner_type (owner_id, type),
    ADD CONSTRAINT chk_assets_type CHECK (type IN (0,1,2,3));
