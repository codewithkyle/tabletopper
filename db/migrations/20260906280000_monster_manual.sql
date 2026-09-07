-- migrate:up
-- THE MONSTER MANUAL. A game master's bestiary: the stat blocks they intend to
-- run, kept per account, read by the VTT when it spawns one.
--
-- THE DROP IS DELIBERATE AND LOSES NOTHING. The monsters table was created
-- 2026-02-27 as a copy of the old SPA's shape and has never been written by
-- anything -- `grep -rn monsters server/sql` finds no statement, and sqlc.yaml
-- says so in as many words. Every environment's copy is empty, and this
-- branch's migrations run from an empty schema, so DROP and CREATE is a rewrite
-- of a shape, not a deletion of data. Altering it column by column would be
-- twenty-odd ALTERs that end at the same table and read as if the old one
-- mattered.
--
-- WHAT CHANGED, AND WHY, AGAINST THE 2024 STAT BLOCK:
--
--   * initiative_bonus is new. The block prints `Initiative +2 (12)`: the
--     Dexterity modifier plus this misc bonus, and ten plus that for the
--     passive. Same shape and same name as characters.initiative_bonus.
--
--   * saving_throws and skills stop being free text and become the character
--     sheet's four JSON blobs -- a misc bonus per entry and a proficiency state
--     per entry. The ability table prints MOD and SAVE columns instead of a
--     separate Saving Throws line, and Skills lists only the entries with a
--     state or a bonus. The arithmetic is the sheet's bonusRows, unchanged.
--
--   * ac loses its parenthetical and becomes a number; the armour that used to
--     sit in the brackets moves to the new gear line, which is what the 2024
--     book does with it.
--
--   * senses holds only the special senses (`Darkvision 60 ft.`). The passive
--     Perception that ends the line is derived from the Perception skill.
--
--   * xp is gone. It follows from cr, and so does the proficiency bonus, so
--     both are looked up at render time from one thirty-four entry table in Go.
--     A monster with lair actions is one rating harder in its lair, and that
--     figure is the next entry in the same table.
--
--   * legendary_action_uses and legendary_action_uses_in_lair carry the counts
--     behind the book's own opening sentence. The in-lair count is zero when it
--     does not change in the lair, and the sentence drops the parenthetical.
--
--   * subtype becomes tags, which is the word both editions use for the
--     parenthetical after the creature type.
--
--   * habitat and treasure are the two lines the 2024 book prints under the
--     block. description is the GM's own paragraph -- lore, tactics, what it
--     wants -- and stands in for the book's descriptive text.
--
-- LAIR ACTIONS AND REGIONAL EFFECTS ARE KEPT even though the 2024 book folded
-- lairs into "in Lair" riders and dropped regional effects entirely. The 2014
-- book and most published adventures have both, and a GM converting older
-- material needs somewhere to put them.
--
-- NOTHING DERIVED IS STORED, which is the rule 20260906250000 established for
-- characters and it applies here without modification. Proficiency bonus and XP
-- follow from cr; the six modifiers follow from the scores; every save and skill
-- total follows from a score, a state, a misc bonus and the proficiency bonus;
-- passive Perception follows from Perception; initiative follows from dex. Every
-- one of those reads columns two different panels own, and the editor autosaves
-- one panel at a time -- so a stored total would go stale the moment the other
-- panel saved.
--
-- THE INSTANCE IS NOT HERE EITHER. This table holds the stat block as printed:
-- maximum hit points, no current hit points, no position, no conditions, no
-- initiative roll. When a room spawns a monster the pawn carries this row's ULID
-- and its own instance stats in the room's state. Editing a monster after it has
-- been spawned changes the book and not the fight.
--
-- cr IS A VARCHAR rather than an ENUM because a select posts it and the
-- validator has to run in Go anyway to normalise what arrives -- which is how
-- spells.school and attacks.damage_type already handle a closed set. The
-- thirty-four values are `0`, `1/8`, `1/4`, `1/2` and `1` through `30`.
--
-- ac is TINYINT UNSIGNED: no printed AC exceeds 25, and the parse helper's range
-- check turns 256 into a validation message rather than a driver error. hp is
-- SMALLINT UNSIGNED, because the Tarrasque has 697.
--
-- EVERY COLUMN BUT THE THREE IDENTITY COLUMNS HAS A DEFAULT, description
-- included -- TEXT cannot take a literal default but takes an expression one.
-- That is what lets CreateMonsterFromName name three columns and nothing else,
-- so creating a monster cannot carry stat-block data.
--
-- One index. (owner_id, name) covers every owner-scoped read and is the list's
-- sort order, because a manual is alphabetical.
DROP TABLE IF EXISTS monsters;

CREATE TABLE monsters (
    id VARBINARY(16) NOT NULL PRIMARY KEY,
    owner_id VARBINARY(16) NOT NULL,
    asset_id VARBINARY(16) NULL,

    name VARCHAR(128) NOT NULL,
    size VARCHAR(32) NOT NULL DEFAULT 'medium',
    type VARCHAR(32) NOT NULL DEFAULT 'humanoid',
    tags VARCHAR(128) NOT NULL DEFAULT '',
    alignment VARCHAR(32) NOT NULL DEFAULT 'unaligned',

    ac TINYINT UNSIGNED NOT NULL DEFAULT 10,
    hp SMALLINT UNSIGNED NOT NULL DEFAULT 1,
    hit_dice VARCHAR(64) NOT NULL DEFAULT '',
    speed VARCHAR(128) NOT NULL DEFAULT '30 ft.',
    initiative_bonus SMALLINT NOT NULL DEFAULT 0,
    cr VARCHAR(4) NOT NULL DEFAULT '0',
    legendary_action_uses TINYINT UNSIGNED NOT NULL DEFAULT 0,
    legendary_action_uses_in_lair TINYINT UNSIGNED NOT NULL DEFAULT 0,

    `str` TINYINT UNSIGNED NOT NULL DEFAULT 10,
    dex TINYINT UNSIGNED NOT NULL DEFAULT 10,
    `con` TINYINT UNSIGNED NOT NULL DEFAULT 10,
    `int` TINYINT UNSIGNED NOT NULL DEFAULT 10,
    wis TINYINT UNSIGNED NOT NULL DEFAULT 10,
    cha TINYINT UNSIGNED NOT NULL DEFAULT 10,

    skills JSON NOT NULL DEFAULT (JSON_OBJECT()),
    skill_proficiencies JSON NOT NULL DEFAULT (JSON_OBJECT()),
    saving_throws JSON NOT NULL DEFAULT (JSON_OBJECT()),
    saving_throw_proficiencies JSON NOT NULL DEFAULT (JSON_OBJECT()),

    vulnerabilities VARCHAR(512) NOT NULL DEFAULT '',
    resistances VARCHAR(512) NOT NULL DEFAULT '',
    immunities VARCHAR(512) NOT NULL DEFAULT '',
    gear VARCHAR(512) NOT NULL DEFAULT '',
    senses VARCHAR(255) NOT NULL DEFAULT '',
    languages VARCHAR(255) NOT NULL DEFAULT '',

    habitat VARCHAR(255) NOT NULL DEFAULT '',
    treasure VARCHAR(64) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT (''),

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    KEY idx_monsters_owner_name (owner_id, name)
);

-- TRAITS AND ACTIONS ARE ROWS, NOT SEVEN JSON BLOBS, which is where the old
-- shape kept them. The character sheet's Features list stayed a blob because it
-- is two fields nobody links to; attacks became rows because a row referenced
-- from a second view needs an identity that survives an edit. The VTT is that
-- second view here -- clicking a pawn and rolling its Bite is the whole reason
-- the Bite is stored -- so each row gets a ULID, its own autosaving form and its
-- own hx-delete, exactly as an attack row has.
--
-- kind IS AN ENUM, unlike monsters.cr, because no select ever posts it: it
-- arrives in the path, is matched against a Go allowlist before any statement
-- runs, and the seven members are closed by the stat block's own format. That is
-- the shares.resource_type situation and not the attacks.damage_type one.
--
-- THE MEMBER ORDER IS THE STAT BLOCK'S ORDER, and that is load-bearing. ENUM
-- members sort by definition order, so `ORDER BY kind, id` comes back as traits,
-- actions, bonus actions, reactions, legendary actions, lair actions, regional
-- effects, and idx_monster_actions_monster serves it without a filesort.
-- Reordering these members later would silently reorder every stat block.
--
-- No `position` column, for the reason inventory has none: ULIDs sort
-- lexicographically by creation time, so ORDER BY id is insertion order and
-- Multiattack goes first because it is written first.
--
-- owner_id is denormalised the way attacks.owner_id is, so a single-row write
-- filters on id, monster_id and owner_id together with no join, and the insert
-- is INSERT ... SELECT off the monsters row so a monster that is not this user's
-- inserts nothing.
--
-- A regional effect has a name the way the others do ("Fog", "Tremors"). The
-- book prints them as an unnamed list, and a blank name renders as a bare
-- paragraph.
CREATE TABLE monster_actions (
    id VARBINARY(16) NOT NULL PRIMARY KEY,
    owner_id VARBINARY(16) NOT NULL,
    monster_id VARBINARY(16) NOT NULL,
    kind ENUM(
        'trait', 'action', 'bonus_action', 'reaction',
        'legendary_action', 'lair_action', 'regional_effect'
    ) NOT NULL,

    name VARCHAR(128) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT (''),

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    KEY idx_monster_actions_monster (monster_id, kind, id)
);

-- A MONSTER'S IMAGE IS ITS OWN MEMBER, NOT `token`. token is reserved for
-- one-off token images placed on a map that belong to no monster; this one is
-- the picture the manual card, the editor bar and eventually the pawn are all
-- drawn from. Appended, never reordered, which is the rule the account-settings
-- ENUMs were added under -- MySQL stores an ENUM by index, so moving a member
-- rewrites the meaning of every row already using the ones after it.
ALTER TABLE assets
    MODIFY `type` ENUM('map', 'avatar', 'token', 'music', 'journal', 'monster') NOT NULL DEFAULT 'map';

-- migrate:down
-- THE ROWS GO BEFORE THE ENUM DOES, for the reason 20260906260000 gives:
-- narrowing an ENUM that still has rows in the member being dropped does not
-- fail, it rewrites them to the empty string and leaves asset rows no statement
-- can reach and the sweeper cannot collect.
DELETE FROM assets WHERE `type` = 'monster';

ALTER TABLE assets
    MODIFY `type` ENUM('map', 'avatar', 'token', 'music', 'journal') NOT NULL DEFAULT 'map';

DROP TABLE IF EXISTS monster_actions;

-- Down restores the 2026-02-27 shape as 20260905180300 left it: snake_case
-- column names and idx_monsters_owner, which is what the migration before this
-- one would roll back from. It restores no rows, and there were none to restore.
DROP TABLE IF EXISTS monsters;

CREATE TABLE monsters (
    id VARBINARY(16) NOT NULL PRIMARY KEY,
    owner_id VARBINARY(16) NOT NULL,
    asset_id VARBINARY(16) NOT NULL,

    name VARCHAR(255) NOT NULL,
    size VARCHAR(32) NOT NULL,
    type VARCHAR(64) NOT NULL,
    subtype VARCHAR(128) NOT NULL,
    alignment VARCHAR(64) NOT NULL,

    ac INT NOT NULL,
    hp INT NOT NULL,
    hit_dice VARCHAR(64) NOT NULL,

    `str` TINYINT UNSIGNED NOT NULL,
    dex TINYINT UNSIGNED NOT NULL,
    `con` TINYINT UNSIGNED NOT NULL,
    `int` TINYINT UNSIGNED NOT NULL,
    wis TINYINT UNSIGNED NOT NULL,
    cha TINYINT UNSIGNED NOT NULL,

    languages VARCHAR(255) NOT NULL,
    cr VARCHAR(4) NOT NULL,
    xp INT UNSIGNED NOT NULL,

    speed VARCHAR(128) NOT NULL,

    vulnerabilities VARCHAR(512) NOT NULL,
    resistances VARCHAR(512) NOT NULL,
    immunities VARCHAR(512) NOT NULL,
    senses VARCHAR(512) NOT NULL,

    saving_throws VARCHAR(512) NOT NULL,
    skills VARCHAR(512) NOT NULL,

    abilities JSON NOT NULL,
    actions JSON NOT NULL,
    bonus_actions JSON NOT NULL,
    legendary_actions JSON NOT NULL,
    reactions JSON NOT NULL,
    lair_actions JSON NOT NULL,

    KEY idx_monsters_owner (owner_id),
    KEY idx_monsters_owner_name (owner_id, name)
);
