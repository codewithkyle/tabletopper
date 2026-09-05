-- migrate:up
-- Every other column in the schema is snake_case; these three were camelCase,
-- and sqlc turned them into Bonusactions and Legendaryactions. The index on
-- owner_id was named as if the column were user_id.
ALTER TABLE monsters
    RENAME COLUMN bonusActions TO bonus_actions,
    RENAME COLUMN legendaryActions TO legendary_actions,
    RENAME COLUMN lairActions TO lair_actions,
    RENAME INDEX idx_monsters_user_id TO idx_monsters_owner;

-- migrate:down
ALTER TABLE monsters
    RENAME COLUMN bonus_actions TO bonusActions,
    RENAME COLUMN legendary_actions TO legendaryActions,
    RENAME COLUMN lair_actions TO lairActions,
    RENAME INDEX idx_monsters_owner TO idx_monsters_user_id;
