-- migrate:up
ALTER TABLE scenes RENAME COLUMN keep_changes TO autosave;

-- migrate:down
ALTER TABLE scenes RENAME COLUMN autosave TO keep_changes;
