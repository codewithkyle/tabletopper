# Monster Manual

A game master's bestiary: the monsters they intend to run, written up as stat blocks
that cover everything the printed Monster Manual prints, kept per account, and ready
for the VTT to spawn when it exists. It is the character sheet's sibling and is built
the way the character sheet was built -- a roster page, a one-question create dialog,
an editor that autosaves a panel at a time, and row-per-entity tables where the row is
the unit of work -- with one thing the sheet does not have: a rendered stat block that
sits beside the editor and redraws after every save.

This document is temporary and is deleted when the work lands. Nothing that ships may
reference it.

## What exists already

- A `monsters` table, created 2026-02-27 as a copy of the old SPA's shape and
  snake_cased on 2026-09-05. **Nothing has ever written it.** `grep -rn monsters
  server/sql` finds no statement, and sqlc.yaml says so in as many words. It is a 2014
  stat block (`saving_throws` and `skills` as free text, `xp` typed by hand, six JSON
  blobs for traits and actions, `asset_id NOT NULL`, no timestamps) and is replaced
  whole in Phase 0 rather than altered column by column.
- A generated `queries.Monster` model, which `make sqlc` regenerates from the new schema.
- `assets.type` has a `token` member. **It is not for this feature.** It is reserved for
  one-off token images placed in the VTT that belong to no monster. A monster's image
  is a new `monster` member, appended by the Phase 0 migration, and `GetImage` in
  `server/sql/assets.sql` gains it beside `map`, `avatar` and `token` -- that
  statement deliberately serves any signed-in user, because those images are shown to
  every player at the table, and a monster's image is too.
- The homepage already links to `/monsters`, which 404s today. Phase 2 makes it real
  and the homepage does not change.
- Everything the editor reuses: `savingPanel` and `panelTrigger` in
  `sheet-section.templ`, `PanelFormErrors`, the field components in
  `form-field.templ`, `abilityField`, `skillsTable`, `savingThrowsTable`,
  `derivedValue`, `proficiencyOptions`, `alignmentOptions`, `sizeOptions`, and in
  `controllers/rules.go` `abilityModifier`, `proficiencyGrant` and `bonusRows`. The
  panel helpers `parsePanelForm`, `renderPanelBlock` and the `recordingDB` test
  harness in `character-panels_test.go` carry over unchanged.
- The old app's stat block and editor, for reference only: `git show
  0d2b584^:server/views/stubs/windows/monster.html` and
  `0d2b584^:server/views/stubs/tabletop/create-monster.html`. The old bundled SRD data
  (`git show 5fe31f5^:public/monsters.ndjson`, 332 rows) is 2014-edition and is not
  imported; see Deferred.

## Decisions

**The manual holds templates. Instances live in the room.** A monster here is the stat
block as printed: maximum hit points, not current; no position, no conditions, no
initiative roll. When a room spawns one, the pawn carries the monster's ULID and its
instance stats -- current hit points, position, conditions -- in the room's own state,
not in any table. The manual is read by id at spawn and whenever a stat block is
opened, and that is the whole of the VTT's dependency on it. It is why there is no
`current_hp`, no vitals panel and no death-save tracker on this sheet, why editing a
monster after it has been spawned changes the book and not the fight, and why the
stat-block fragment takes the monster's id: it is the lookup the VTT will make. A pawn
whose monster has since been deleted keeps its instance stats and loses its stat block;
what it renders as is the VTT's decision.

**The stat block covers everything the printed one does.** The 2024 layout is the
spine, and the sections the 2024 book moved out of the block are kept anyway, because
a GM converting older material needs somewhere to put them. What that means for the
columns:

- An **Initiative** line: modifier and passive score, `Initiative +2 (12)`. The
  modifier is the Dexterity modifier plus a stored misc bonus, the passive is ten plus
  that. Same shape as `characters.initiative_bonus`.
- The ability table has **MOD and SAVE columns**, so there is no separate "Saving
  Throws" line. A save is the modifier plus what a proficiency state grants plus a misc
  bonus -- which is exactly the character sheet's saving-throw grid, so the columns
  are the same four JSON blobs (`skills`, `skill_proficiencies`, `saving_throws`,
  `saving_throw_proficiencies`) and the arithmetic is `bonusRows`, unchanged.
- **Skills** is a line listing only the rows with a proficiency state or a misc bonus:
  `Skills Perception +5, Stealth +6`. Same grid, filtered at render.
- The AC line lost its parenthetical; armour moved to a **Gear** line. So `ac` is a
  number and there is a `gear` text column.
- Damage and condition immunities are one **Immunities** line. Free text, comma lists,
  semicolon between the two halves, which is how the book prints it.
- **Senses** ends in the passive Perception, which is derived: ten plus the Perception
  skill total. The column holds only the special senses (`Darkvision 60 ft.`).
- **CR** prints its derived numbers beside it: `CR 5 (XP 1,800; PB +3)`. A monster with
  lair actions is one rating harder in its lair, so the line grows the next rating's
  XP: `CR 17 (XP 18,000, or 20,000 in lair; PB +6)`. None of it is stored; the `xp`
  column goes.
- **Legendary Actions** open with the book's sentence, built from a count and an
  optional in-lair count: `Legendary Action Uses: 3 (4 in Lair).` Both counts are
  columns; the sentence is rendered.
- **Lair Actions and Regional Effects are sections of their own.** The 2024 book
  folded lairs into "in Lair" riders and dropped regional effects; the 2014 book and
  most published adventures have both, and the old table had lair actions. They are
  two more members of the action-kind ENUM and two more sections on the page, each
  opening with the book's own sentence. Nothing else about the design changes for them.
- **Habitat and Treasure**, the two lines the 2024 book prints under the stat block,
  are two word columns on the Description panel.

**Derive, never store.** Proficiency bonus and XP follow from CR; the six modifiers
follow from the scores; every save and skill total follows from a score, a state, a
misc bonus and the proficiency bonus; passive Perception follows from Perception;
initiative follows from Dexterity; in-lair XP follows from CR and the presence of lair
actions. Every one of those depends on columns owned by two different panels, and the
editor autosaves one panel at a time, so a stored value would go stale the moment the
other panel saved. This is the character sheet's rule
(`20260906250000_derive_character_bonuses.sql`) and it applies without modification.

**CR is a VARCHAR and a Go allowlist of thirty-four values** -- `0`, `1/8`, `1/4`,
`1/2`, `1` through `30` -- which is how `spells.school` and `attacks.damage_type`
already handle a closed set that a select posts. One slice in `pages` carries the
value, the label, the XP and the proficiency bonus of each rating, so the select, the
validator and the arithmetic read the same thirty-four lines and cannot drift. The
in-lair XP is the next entry in that slice; CR 30 has no next entry and prints none.

CR 0 is the one rating with two XP values in the rules: 0 for a creature with no
damaging attack, 10 for one that has. The row does not store which; the stat block
answers 10 when the monster has at least one row of kind `action` and 0 otherwise.

**Traits and actions are rows in one table, not seven JSON blobs.** The sheet kept
Features as a blob because it is two fields and the whole list is small; attacks became
rows because "a row referenced from a second view needs an identity that survives an
edit". The VTT is that second view for monsters -- clicking a pawn and rolling its
Bite is the whole point of storing the Bite -- so `monster_actions` is a table, with a
`kind` column naming which of the seven sections a row belongs to. Each row is its own
autosaving form and deletes itself with `hx-delete`, exactly as an attack row does.
Rows order by id, which is insertion order; Multiattack goes first because it is
written first.

**Free text for speed, senses, languages, gear and the three defense lines.** Every one
of them has structure in the rules and a closed vocabulary behind it, and every one of
them is written as a sentence in the book. Structured damage types would let a future
VTT apply damage automatically; that future is not this feature, and fifty-four
checkboxes on the editor would be its cost today. The character sheet's `speed` is free
text for the same reason and `splitMeasurement` already pulls the leading number off it
when a chip wants one.

**Size is one of the six, not "Small or Medium".** The 2024 book prints dual sizes on a
handful of shapechangers. The VTT will read `size` to decide a pawn's footprint --
Tiny, Small and Medium one square, Large four, Huge nine, Gargantuan sixteen -- and a
footprint has to be one answer. Deferred, with the note that the column is already text.

**Alignment reuses `alignmentOptions` and renders Unaligned.** The character subtitle
hides "unaligned" because nobody chose it; a beast's stat block prints it because the
book does. The difference is in the two subtitle functions and nowhere else.

**The image is an `assets` row of type `monster`, 256 pixels square, WebP,** at
`users/{userID}/monsters/{assetID}`. It is what the pawn will be drawn with, so it is
not the `token` type -- that member is for one-off token images that belong to no
monster -- and not the 96-pixel `avatarSize`, because a portrait is a thumbnail and a
pawn is drawn on a map at whatever zoom the GM likes. The upload lives on the card on
the manual page, where the character's avatar upload lives, and the editor bar shows
the image without offering to change it. Same handler shape as
`UploadCharacterAvatar`, same row-before-object ordering, same compensating delete,
same replace-at-the-same-key rule so nothing is ever orphaned and nothing needs
sweeping.

**One stat block component, rendered three ways.** In the editor as the sticky left
column; out-of-band after every save so it redraws live; and as a `/fragment/` route
for the content modal on the manual page, which is also the URL the VTT will open when
a GM clicks a pawn. The fragment rule says a fragment is never a second copy of markup,
and here that is what makes the preview trustworthy: what the GM sees while editing is
byte-for-byte what the table will see.

**No tabs.** A stat block is one screen. The editor is `/monsters/{id}/edit` and
nothing under it. The bar across the top is the character bar's shape -- image, name,
subtitle, Back -- with its own chips (AC, HP, Speed, Initiative, PB, CR), built by a
`monsterHeader` function so the chips and the stat block cannot disagree.

**Search is the journal's search.** A `type="search"` box on the manual page hitting
`/fragment/monster/list?q=` on a debounce, returning the same `monsterCards`
component the page rendered, filtered by `name LIKE` with the term escaped by the
function journal search already has. The term is capped at the name column's length
and a bad request is a 404 with an empty body. The list sorts by name, because a
manual is alphabetical; `idx_monsters_owner_name` serves it.

**Rebuild the table rather than alter it.** The migration is `DROP TABLE monsters` and
a fresh `CREATE TABLE`, and the down recreates the 2026-02-27 shape. No environment
has a row to lose: no statement has ever written the table, and the branch's
migrations start from an empty schema. The comment in the migration says exactly
that, so the next person does not read a DROP as carelessness.

## Schema

### `monsters`

```sql
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
```

Every column but the three identity columns has a default, which is what lets
`CreateMonsterFromName` take three values and nothing else -- the property
`TestCreateCannotCarrySheetData` pins for characters, and stronger here because no
literal is needed in the statement at all.

`ac` is `TINYINT UNSIGNED`: no AC in print exceeds 25 and the parse helper's range
check turns 256 into a validation message rather than a driver error. `hp` is
`SMALLINT UNSIGNED`; the Tarrasque has 697. The scores are `TINYINT UNSIGNED` like the
character's. `description` is the GM's own paragraph -- lore, tactics, what it wants --
and stands in for the book's descriptive text; it is TEXT with an expression default
for the reason the personality columns are. `habitat` and `treasure` are the two lines
the book prints under the block and are words, not paragraphs.

`legendary_action_uses_in_lair` is zero when the count does not change in the lair,
and the sentence omits the parenthetical then. `subtype` becomes `tags`, which is the
word both editions use for the parenthetical after the type. Only the one index:
`(owner_id, name)` covers every owner-scoped read and is the list's sort order.

### `monster_actions`

```sql
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
```

`kind` is an ENUM rather than a VARCHAR because no select ever posts it: it arrives in
the path, is matched against a Go allowlist before any statement runs (the
`bonusPanels` shape), and the seven members are closed by the format. That is the
`shares.resource_type` situation, not the `attacks.damage_type` one. ENUM members sort
by definition order, so `ORDER BY kind, id` is stat-block order -- traits, actions,
bonus actions, reactions, legendary actions, lair actions, regional effects -- and the
index serves it without a filesort. A regional effect has a name the way the others do
("Fog", "Tremors"); the book prints them as a list without names, and a blank name
renders as a bare paragraph.

`owner_id` is denormalised the way `attacks.owner_id` is, so a single-row write filters
on all three of id, monster and owner without a join, and the insert is `INSERT ...
SELECT` off the monsters row so a monster that is not this user's inserts nothing.

### `assets.type`

```sql
ALTER TABLE assets
    MODIFY `type` ENUM('map', 'avatar', 'token', 'music', 'journal', 'monster') NOT NULL DEFAULT 'map';
```

Appended, never reordered, which is the rule the account-settings ENUMs were added
under. The down deletes the `monster` rows before narrowing the ENUM, for the reason
`20260906260000_share_character_resource.sql` gives: narrowing with rows still in the
member does not fail, it rewrites them to the empty string and leaves rows no
statement can reach.

### `sqlc.yaml`

- Add `"cr"` to `initialisms`, so the field is `CR` and not `Cr`.
- Add `- column: "*.monster_id"` with the non-null `*ulid` type. It matches none of the
  existing wildcards and would otherwise come back as `[]byte`.
- Replace the paragraph about `monsters.asset_id` needing an entry: the column is
  nullable now, so the nullable wildcard that reaches it is the right one, and the
  comment should say that instead.

### Migration order

`db/migrations/<stamp>_monster_manual.sql` with the two tables and the ENUM. Then
`make db` (dbmate applies it and dumps `db/schema.sql`), then `make sqlc`. The schema
dump is what the schema-driven tests read, so it has to be regenerated before
Phase 3's tests can pass.

## Derived values

New in `controllers/rules.go`, beside the character arithmetic:

- `challengeRating(value)` looks a CR up in `pages.ChallengeRatings()` and returns its
  XP, its proficiency bonus, and the XP of the next rating for the in-lair figure.
  Proficiency: CR 0-4 is +2, then +1 per four ratings, to +9 at CR 29-30. XP from the
  book's table (0, 25, 50, 100, 200, 450, 700, 1,100, 1,800, 2,300, 2,900, 3,900,
  5,000, 5,900, 7,200, 8,400, 10,000, 11,500, 13,000, 15,000, 18,000, 20,000, 22,000,
  25,000, 33,000, 41,000, 50,000, 62,000, 75,000, 90,000, 105,000, 120,000, 135,000,
  155,000).
- `monsterDerived(monster, actions)` returns a `pages.MonsterDerived`: the six modifiers,
  both bonus grids as `[]BonusRow` via `bonusRows`, passive Perception, initiative and
  passive initiative, proficiency bonus, XP, and in-lair XP. It takes the actions for
  the CR 0 rule and the lair rule, both of which read only the kinds present.
- `monsterStatBlock(monster, actions, derived)` builds the `pages.StatBlock` every
  render reads. It is the privacy-boundary shape `sharedCharacterSheet` has -- every
  value a string the controller wrote down by name -- because the same struct is what
  the VTT will one day hand to players.
- `monsterHeader(monster, derived)` builds the bar.
- `monsterSubtitle`: `Small Humanoid (Goblinoid), Chaotic Neutral`. Tags in parentheses
  only when present; alignment through `AlignmentLabel`, Unaligned included.

New in `templ/pages/monster-options.go`:

- `creatureTypeOptions`: the fourteen types (Aberration, Beast, Celestial, Construct,
  Dragon, Elemental, Fey, Fiend, Giant, Humanoid, Monstrosity, Ooze, Plant, Undead),
  lower-case values, `NormalizeCreatureType` falling back to `DefaultCreatureType`.
- `ChallengeRating{Value, Label string; XP uint32; Proficiency uint8}` and
  `ChallengeRatings()`, `NormalizeChallengeRating` falling back to `"0"`.
- `MonsterActionKind` constants and `monsterActionKinds` -- the allowlist, mapping the
  path word to the section heading, the add button's label and the section's opening
  sentence if it has one (`trait` / "Traits" / "Add Trait" / none; `legendary_action`
  / "Legendary Actions" / "Add Legendary Action" / the uses sentence; `lair_action` /
  "Lair Actions" / "Add Lair Action" / the initiative-count-20 sentence;
  `regional_effect` / "Regional Effects" / "Add Regional Effect" / the warped-region
  sentence).
- `MonsterNameLimit = 128` and the other column widths the handlers and the
  `maxlength` attributes both read, the way `vitals.go` holds the vitals bounds.

The three opening sentences are the book's, with "it" where the book names the
creature: "Immediately after another creature's turn, it can expend a use to take one
of the following actions. It regains all expended uses at the start of each of its
turns." A stat block has no short noun to put there -- the book writes "the dragon"
under a heading that says "Ancient Red Dragon" -- and a sentence reading "the Ancient
Red Dragon can expend a use" is worse than the pronoun.

## Routes

```go
mux.HandleFunc("GET /monsters", auth.RequireSession(app.MonstersPage))
mux.HandleFunc("POST /monsters", auth.RequireSession(app.NewMonsterForm))
mux.HandleFunc("GET /monsters/{id}/edit", auth.RequireSession(app.MonsterPage))
mux.HandleFunc("DELETE /monsters/{id}", auth.RequireSession(app.DeleteMonster))
mux.HandleFunc("POST /monsters/{id}/image", auth.RequireSession(app.UploadMonsterImage))

mux.HandleFunc("POST /monsters/{id}/identity", auth.RequireSession(app.SaveMonsterIdentity))
mux.HandleFunc("POST /monsters/{id}/abilities", auth.RequireSession(app.SaveMonsterAbilities))
mux.HandleFunc("POST /monsters/{id}/combat", auth.RequireSession(app.SaveMonsterCombat))
mux.HandleFunc("POST /monsters/{id}/defenses", auth.RequireSession(app.SaveMonsterDefenses))
mux.HandleFunc("POST /monsters/{id}/bonuses/{kind}", auth.RequireSession(app.SaveMonsterBonuses))
mux.HandleFunc("POST /monsters/{id}/description", auth.RequireSession(app.SaveMonsterDescription))

mux.HandleFunc("POST /monsters/{id}/actions/{kind}", auth.RequireSession(app.AddMonsterAction))
mux.HandleFunc("POST /monsters/{id}/actions/{kind}/{actionId}", auth.RequireSession(app.SaveMonsterAction))
mux.HandleFunc("DELETE /monsters/{id}/actions/{kind}/{actionId}", auth.RequireSession(app.DeleteMonsterAction))

mux.HandleFunc("GET /fragment/monster/new", auth.Fragment(app.NewMonsterFragment))
mux.HandleFunc("GET /fragment/monster/list", auth.Fragment(app.MonsterListFragment))
mux.HandleFunc("GET /fragment/monster/stat-block", auth.Fragment(app.MonsterStatBlockFragment))
```

The kind rides in the action routes' path for the reason the spell level rides in the
spell routes': a row cannot change kind, so it identifies the row as much as the id
does, and it is what lets the add know which section to append to. Every literal at
the third segment (`edit`, `image`, `identity`, `abilities`, `combat`, `defenses`,
`bonuses`, `description`, `actions`) is distinct from every other and no wildcard
sits there, so the mux has nothing to disambiguate. `TestPanelRoutesMatchTheirOwnPatterns`
grows a block of monster cases, including that `POST /fragment/monster/list` falls to
the `/fragment/` subtree.

`GET /fragment/monster/stat-block?monster={id}` reads the id from the query string
rather than a path for the reason the share dialogs do: it is not the monster's URL, it
is a representation of it that the manual page opens in a dialog. It is also the shape
of the lookup a pawn will make, since a pawn holds nothing but this id and its own
instance stats. When the VTT exists it will need a second stat-block route scoped to
room membership rather than ownership; that is a route for that work, and this one
stays owner-scoped.

## Panels

Every editable column belongs to exactly one panel, and a schema-driven test holds
that line the way `TestPanelsCoverEveryEditableColumn` does for characters.

| Panel | Route | Columns |
| --- | --- | --- |
| Identity | `/identity` | name, size, type, tags, alignment |
| Abilities | `/abilities` | str, dex, con, int, wis, cha |
| Combat | `/combat` | ac, hp, hit_dice, speed, initiative_bonus, cr, legendary_action_uses, legendary_action_uses_in_lair |
| Saving Throws | `/bonuses/saving_throws` | saving_throws, saving_throw_proficiencies |
| Skills | `/bonuses/skills` | skills, skill_proficiencies |
| Defenses & Senses | `/defenses` | vulnerabilities, resistances, immunities, gear, senses, languages |
| Description | `/description` | habitat, treasure, description |

Unowned, and checked to stay unowned: `id`, `owner_id`, `asset_id` (the image
route's), `created_at`, `updated_at`.

The two legendary counts sit in Combat rather than beside the legendary action rows
because those rows are forms and forms cannot nest -- the section holding them is a
`sheetPanel`, not a `savingPanel`, exactly as Attacks is on the character sheet.

Each save answers the way `finishCharacterPanel` does -- toast, cleared error block --
and then re-reads the row and the actions and renders three out-of-band swaps:
`MonsterDerivedValues` (the modifiers, both grids' totals, passive Perception),
`MonsterBarValues` (the chips), and the stat block. An action row's save and delete
render the stat block swap too, which is what keeps the preview live while a GM types
a Bite -- and what makes the in-lair XP appear the moment the first lair action is
added.

### Editor layout

`grid-cols-[24rem_minmax(0,1fr)]`, the character page's split. The left column is
sticky and holds the rendered stat block. The right column, top to bottom: Identity,
Abilities, Combat, then Saving Throws and Skills side by side, Defenses & Senses, then
the seven action sections (Traits, Actions, Bonus Actions, Reactions, Legendary
Actions, Lair Actions, Regional Effects) each a `sheetPanel` of rows with an add
button, then Description. A section's opening sentence, where it has one, renders
under the heading.

An action row is `MonsterActionRow(monsterID, kind, row)`: name input, description
textarea, Delete, its own `hx-post` on `input delay:1s` and its own error block,
copied from `AttackRow` with two fields instead of six.

### Stat block markup

`MonsterStatBlock(sb StatBlock, oob bool)` in `templ/pages/stat-block.templ`. An
`<article id="stat-block">` carrying `hx-swap-oob="true"` when `oob` is set, the way
`characterBarFigure` does it. Sections in book order:

1. Image (or the initial), name, subtitle.
2. `AC` · `Initiative +N (P)` / `HP N (dice)` / `Speed ...`.
3. The ability table: two columns of three, each row `STR 8 -1 -1` with headers
   Score / Mod / Save.
4. Lines, each omitted when empty: Skills, Vulnerabilities, Resistances, Immunities,
   Gear. Then Senses (always -- it ends in passive Perception), Languages ("None" when
   blank), CR with XP, in-lair XP when there are lair actions, and PB.
5. Traits, Actions, Bonus Actions, Reactions, Legendary Actions, Lair Actions, Regional
   Effects, each omitted when it has no rows, each with its opening sentence where it
   has one. Descriptions render as plain text with `whitespace-pre-line`.
6. Habitat and Treasure, when set, as the book prints them under the block.

Every number goes through `derivedValue` or is plain text; nothing on it is an input,
so the same component serves the modal and, later, a player.

`MonsterStatBlockFragment(sb)` wraps the block in the content modal's contract: a
heading, the block, and its own `Close` button (`btn border-base-content/50`,
dispatching `modal:close`). The manual page's View button opens it with
`data-modal-open` and `data-modal-size="lg"`, which content-modal.js already reads.

## Phases

Each phase ends with `make check` green and, when a `.templ` file changed, the CSS
selector diff from the project instructions run and every added selector accounted
for. No comment of any kind goes into a `.templ` file; reasoning goes in the handler
or the page-data `.go` file.

### Phase 0 -- Schema

- `db/migrations/<stamp>_monster_manual.sql`: drop and recreate `monsters`, create
  `monster_actions`, append `monster` to `assets.type`, with the reasoning above in the
  migration comment (the DROP, the 2024 lines, the kept sections, the ENUMs, the derive
  rule). Down: delete `monster` asset rows and narrow the ENUM, drop `monster_actions`,
  drop and recreate the 2026-02-27 `monsters`.
- `sqlc.yaml`: the three edits above.
- `make db && make sqlc`. Confirm `queries.Monster` has `CR`, `Tags`, `Habitat`,
  `AssetID *ulid.ULID`, and `queries.MonsterAction.MonsterID` is `ulid.ULID`.
- `TestDeletingACharacterEmptiesEveryTableThatHoldsItsRows` still passes: the new tables
  carry no `character_id`, so the schema scan does not pick them up.

### Phase 1 -- Queries and rules

- `server/sql/monsters.sql`: `ListMonsters` (owner, by name), `SearchMonsters` (owner,
  `name LIKE`), `GetMonster`, `GetMonsterAsset` (name, asset_id, file_path via LEFT JOIN
  assets), `CreateMonsterFromName` (three values), `UpdateMonsterImage`,
  `DeleteMonster`, and the seven panel updates, each `:execresult`, each naming exactly
  its panel's columns.
- `server/sql/monster-actions.sql`: `ListMonsterActions` (monster, owner, `ORDER BY
  kind, id`), `InsertMonsterAction` (`INSERT ... SELECT` off monsters, four values: id,
  monster, owner, kind), `GetMonsterAction`, `UpdateMonsterAction`,
  `DeleteMonsterAction`, `DeleteMonsterActions` (the purge step).
- `assets.sql`: `InsertMonsterImage`, the `InsertAvatar` shape with `'monster'`;
  `GetImage` gains `'monster'` in its IN list, with a line in its comment saying the
  image is shown to every player the way a map is.
- `storage/keys.go`: `MonsterImageKey`, `UploadMonsterImage`.
- `controllers/rules.go`: `challengeRating`, `monsterDerived`, `monsterStatBlock`,
  `monsterHeader`, `monsterSubtitle`, `formatXP` (thousands separators).
- `templ/pages/monster-options.go`, `edit-monster.go` (`EditMonsterPageData`,
  `MonsterHeader`, `MonsterDerived`, `StatBlock`, `StatBlockAbility`, `StatBlockEntry`,
  `StatBlockSection`, `MonsterAction`), `monsters.go` (card helpers).
- Tests: `TestEveryMonsterQueryIsScopedToTheOwner` and
  `TestEveryMonsterActionQueryIsScopedToTheOwnerAndMonster` (the attack test's shape,
  reading the two `.sql` files); `TestEveryChallengeRatingHasItsNumbers` (34 entries,
  XP strictly increasing above CR 0, PB steps where the rules say);
  `TestCombatDerivesProficiencyAndXPFromCR` (spot checks including both CR 0 answers
  and the in-lair figure appearing only with a lair action);
  `TestTheStatBlockOmitsWhatTheMonsterHasNot`, `TestTheSubtitlePrintsUnaligned` and
  `TestEveryActionKindHasASection` in `pages`.

### Phase 2 -- The manual page, create and delete

- `templ/pages/monsters.templ`: `Monsters(...)`, `monsterCards(...)` (the grid, id
  `monster-cards`), `MonsterCard(...)`, `MonsterCardsFragment(...)`. The card: image or
  initial with the upload control, name, subtitle, chips for CR / AC / HP / Speed, and
  View / Edit / Delete. The search box above the grid, wired as the journal's is.
- `templ/pages/new-monster-form.templ`: `NewMonsterFragment()`, the one-field dialog,
  copied from `new-character-form.templ` with its own panel name.
- `controllers/monsters.go`: `MonstersPage`, `MonsterListFragment` (validate `q` length
  against `MonsterNameLimit`, escape it with the journal's helper, 404 empty body on a
  bad request), `NewMonsterFragment`, `NewMonsterForm` (trim, require, cap, insert,
  toast, `HX-Redirect` to the editor), `DeleteMonster` (image object first, then
  `deleteMonsterRows`, then the row, then the asset row logged-not-reported, in
  `DeleteCharacter`'s order and for its reasons), `deleteMonsterRows` (one statement per
  table, today only `monster_actions`), `loadMonster` (the ownership gate, redirecting
  to `/monsters` on a miss).
- `routes.go`: the five page-level routes and the two fragments; comments in the
  routes file for anything that differs from the character block.
- Tests: `TestCreateMonsterCannotCarrySheetData` (3 placeholders, 3 params);
  `TestDeletingAMonsterEmptiesEveryTableThatHoldsItsRows` (scan `db/schema.sql` for
  `monster_id`, the character test's shape); `TestTheMonsterPurgeIsScopedToItsOwner`;
  `TestMonsterListFragmentRefusesAnOverlongTerm`; `TestPagesRenderConcurrently` gains
  `monsters`, `monster-cards-fragment`, `new-monster-fragment`;
  `TestPanelRoutesMatchTheirOwnPatterns` gains the monster block.

### Phase 3 -- The editor and the stat block

- `templ/pages/edit-monster.templ`: `EditMonster(data)`, `monsterBar` (reusing
  `appBar`, `barGutter`, the figure and chips pattern with `hx-swap-oob` ids
  `monster-bar-figure` and `monster-bar-chips`), `monsterPanels(data)`.
- `templ/pages/stat-block.templ`: `MonsterStatBlock`, `MonsterStatBlockFragment`.
- `templ/pages/monster-derived.templ`: `MonsterDerivedValues(d)` -- the six modifiers,
  both grids' totals, passive Perception. Not `DerivedValues`: that one emits the two
  spell ids, which this page does not render.
- `controllers/monsters.go`: `MonsterPage` (row + actions, two queries).
- `controllers/monster-panels.go`: the seven save handlers, their input builders, and
  `finishMonsterPanel`. `SaveMonsterBonuses` reuses `marshalBonusPayloads` and
  `bonusPanels` unchanged -- the field prefixes are the same because the grids are the
  same components.
- `controllers/monsters.go`: `MonsterStatBlockFragment` (parse the id, 404 empty body
  on a bad one, load row and actions, render).
- Tests: `TestMonsterPanelsWriteOnlyTheirOwnColumns`,
  `TestMonsterPanelsCoverEveryEditableColumn` (reading `monsters` out of the schema
  dump, `unownedMonsterColumns` for the five), `TestMonsterPanelValidationFailsBeforeTheWrite`,
  `TestOverlongMonsterFieldsAreRejectedNotTruncated` (every VARCHAR, in characters;
  description in bytes), `TestMonsterSelectsNormaliseAnythingNotOnTheList` (type,
  size, alignment, CR), `TestASaveRedrawsTheStatBlock` (the response carries the
  oob article), `TestStatBlockFragmentRejectsABadID` (no statement runs);
  `TestPagesRenderConcurrently` gains `edit-monster` and `stat-block-fragment`.

### Phase 4 -- Action rows

- `templ/pages/monster-action-row.templ`: `MonsterActionRow`, `monsterActionsTable`
  (rows container id `actions-<kind>`, the add button posting to
  `/monsters/{id}/actions/{kind}` with `hx-swap="append"`), rendered once per kind by
  ranging over `monsterActionKinds` so a section cannot be forgotten.
- `controllers/monster-actions.go`: `AddMonsterAction` (allowlist the kind, insert,
  read back, answer with the row), `SaveMonsterAction`, `DeleteMonsterAction`
  (answers 200, for the reason `DeleteAttack` gives), `finishMonsterActionRow`
  (toast, cleared block, stat block oob), `monsterActionKind` (the allowlist gate,
  `unknownBonusKind`'s shape).
- Tests: `TestAddMonsterActionCannotCarryActionData` (four values, all ids or the kind,
  `FROM monsters` in the statement, four params), `TestAnUnknownActionKindNeverBecomesAStatement`,
  `TestDeleteMonsterActionAnswers200SoTheRowIsSwappedOut`,
  `TestMissingActionRowIsAnAction404`, `TestUnparseableActionIDTouchesNoDatabase`,
  `TestAnActionSaveRedrawsTheStatBlock`, `TestTheEditorRendersEverySection`.

### Phase 5 -- Monster image

- `controllers/assets.go`: `monsterImageSize = 256`, `UploadMonsterImage` (the avatar
  handler with `GetMonsterAsset`, `InsertMonsterImage`, `UploadMonsterImage`,
  `UpdateMonsterImage`, `discardMonsterImage`), answering with the re-rendered
  `MonsterCard`.
- The card's upload control and the bar's figure read `asset_id` through
  `/assets/images/{id}`, which serves `monster` rows from Phase 1.
- `DeleteMonster` already deletes the object and the asset row from Phase 2; confirm
  with a test that the object delete precedes the row deletes.
- Tests: `TestAMonsterImageUploadWritesTheRowBeforeReachingR2` (the map upload test's
  shape), `TestMonsterImageRoutesRejectUnparseableIDs`,
  `TestGetImageServesMonsterImagesAndNotJournalOnes` (the IN list, read from
  `assets.sql`).

### Phase 6 -- Verification

- `make check`.
- The CSS selector diff after every `.templ` change; the expected additions are the
  grid and table utilities the stat block introduces and nothing named after a DaisyUI
  component that is not in the markup.
- In the running app (`make run`, then chromium): create a monster from the manual,
  watch the redirect land in the editor, type a CR and see the PB and XP change in the
  block, tick a saving-throw proficiency and see the SAVE column move, add a Bite and
  see it appear in Actions, add a lair action and see the in-lair XP appear on the CR
  line, set the legendary counts and read the sentence, delete a row, upload an image
  from the card, search the manual, open the block in the modal, delete the monster and
  confirm the image object is gone from R2.
- Delete this file in the commit that lands the last phase.

## Deferred

Named so that "we forgot" and "we decided" do not look the same later.

- **Sharing a monster by link.** `shares.character_id` is `NOT NULL` and the purge
  test scans for it; a monster share needs that column nullable or a `monster_id`
  beside it, plus a reader route that serves the image. One migration and the share
  dialog's three strings, when it is wanted.
- **SRD import.** A "Copy from the SRD" picker seeded from the 2024 SRD 5.2 (CC BY
  4.0). The old `public/monsters.ndjson` is 2014 data in the 2014 shape and is not the
  seed.
- **Duplicate a monster.** `POST /monsters/{id}/copy` copying the row and its actions
  and redirecting to the copy. Cheap, and the first thing a GM building a goblin boss
  will ask for.
- **Structured attacks.** To-hit, reach, damage dice and type as columns on an action
  row, so the VTT can roll them. The `description` column stays as the sentence the
  book prints; the structured fields would be beside it, not instead of it.
- **Row reordering.** Insertion order is right until it is not; a `position` column
  and a drag handle when somebody needs Multiattack moved.
- **Dual sizes** ("Small or Medium") once the VTT decides what footprint it means.
- **Structured damage types and conditions** on the three defense lines.
- **Markdown in descriptions**, through `internal/markdown` with images dropped, for the
  italicised *Melee Attack Roll:* the book uses. Plain text until then.
- **Image upload from the editor bar**, in addition to the card.
- **The room-scoped stat block route** and everything else a pawn needs: reading the
  manual by ULID at spawn, instance stats in room state, what a pawn shows when its
  monster is gone. All VTT work, all reading this feature and none of it changing it.
