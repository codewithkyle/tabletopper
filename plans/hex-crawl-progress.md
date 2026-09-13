# Hex crawl: progress

Working notes against `plans/hex-crawl.md`. Delete this file when the plan is done.

Branch `yet-another-rewrite`. Checkpoints 1-7 are committed (`edcae11`, `4f17aed`,
`6ddd4a5`, `7b51195`, `1fb5899`, `bb275db`); **checkpoint 8 is uncommitted, in the
working tree**.
`make check` is green and `make js`, `make css`, `make protocol`, `make sqlc`, `make db`
and `templ generate` have all been run, so `make run` is enough to look at it.

## Where we are

**Checkpoints 1 to 6 are built and driven in a browser.** Checkpoint 7 is committed but
was never reported as driven, and checkpoint 8 is built and unverified. The next session
starts by running the CP7 and CP8 test plans at the bottom of this file. **That is the
whole plan** — there is no checkpoint 9.

## Decisions taken on 2026-09-12

1. **Checkpoints are the plan's phases, with 0, 1 and 2 merged.** Phases 0 and 1 are
   pure model and maths with nothing to look at; folding them into phase 2 makes the
   first checkpoint one you can actually drive.
2. **The `cells` unit reads as `hex` on a hex grid and `sq.` on a square one.** The
   plan's glyph list implied a bare `hex`, which reads wrong under a square grid. This
   costs `s` and `q` in the atlas beyond the plan's `mi`, `km` and `hex`.
3. **The phase 5 enum migration gets applied by me**, so CP4 is ready to test the
   moment it is handed over. `make sqlc` reads schema files, not the database; `make db`
   is the part that touches local docker MySQL.

## Decisions taken on 2026-09-13

4. **The two map slots are two resources, not one with a flag.** `POST`/`DELETE
   /rooms/{id}/layers/{layer}/map` is the players' map and `.../gm-map` is the GM's own.
   A `DELETE` carries no body, and a GM clearing the keyed map must not be one dropped
   query parameter away from clearing the table's. The picker fragments carry `gm=1`,
   because there the slot is only display state deciding where a card posts.
5. **Phase 9 was thrown out and rewritten on the GM's say-so.** It was *the party pawn
   reveals fog as it moves*; it is now *numbering the cells*. Reveal-on-move automates a
   call the GM makes deliberately and fights the fog tool; a reference number is the thing
   a hex crawl cannot be run without, because the GM needs to say a hex out loud and find
   it in their own notes. `Pawn.Party` and `TableSettings.AutoReveal` are not built and
   are not in the plan any more.
6. **A running count, not `0412`.** I argued for column-and-row references, because a
   running count renumbers the whole map whenever its extent changes and a coordinate does
   not. The GM's answer was that there is no prep to break — it is a local app with test
   data. Built as asked. The format is one function in `model/numbering.ts` if it ever
   proves to matter.
7. **The cell brush is a third button in the fog shape group, not a fourth control.**
   `FogOptions` gains `cells: boolean` exactly as the plan says, but the UI renders it
   as `data-fog-shape="cells"` beside Rectangle and Polygon, because a boolean beside a
   radio group leaves "cells + polygon" meaning nothing. `fog-tool.ts` derives the
   pressed button from `options.cells ? "cells" : options.shape`.

## The checkpoints

| | Phases | What you can do at the end of it |
| --- | --- | --- |
| **CP1** | 0, 1, 2 | A hex grid you can see, snap to and measure in. **Verified.** |
| **CP2** | 3 | A layer carries two pictures: the GM's map and the players' map. **Verified.** |
| **CP3** | 4 | The fog brush paints by the cell, square or hex. **Verified.** |
| **CP4** | 5 | Terrain is its own asset kind with its own tab and its own shelf. **Verified.** |
| **CP5** | 6 | The palette, the wheel's ring, one-at-a-time stamping and the tile stage. **Verified.** |
| **CP6** | 7 | Players stamp, behind `PlayersCanStamp`. **Verified.** |
| **CP7** | 8 | The hex key: notes per cell, revealed by the GM. **Committed, unverified.** |
| **CP8** | 9 | The cells are numbered, so the GM can name a hex. **Built, unverified.** |

CP5 is by far the largest. CP1 was three phases only because two of them were invisible.

## What Checkpoint 1 landed

### Phase 0 — the grid grows a type and a unit

- `Grid.Type` is `square | hexPointy | hexFlat`; `Grid.Units` is
  `feet | miles | kilometres | cells`. Both in the `Values()`/`Valid()` idiom, so the
  protocol generator picked them up unaided.
- `Normalize` fills them the way it fills `Snap` and `Lines`. **`room.Schema` stays at
  3 and nothing migrates**, and every later phase in this plan is additive too, so no
  phase here bumps it.
- `FeetPerCell` keeps its name and column. Only the label changed, to
  *Distance per cell*.
- The grid form gained a `gridType` radio group and a `units` select; `gridForm`,
  `gridData` and `RoomGridData` carry both.

### Phase 1 — hex arithmetic

- `server/internal/room/hex.go` and `server/js/room/model/hex.ts`: `hexAt`,
  `hexCentre`, `hexRound`, `hexDistance`, `hexLine`, `hexCorners`.
- `snapPawn` branches on both sides: on a hex grid creatures centre regardless of
  size, `halfCells` behaves as `cells`, and `off` and objects pass through.
- `cellAt` / `cellCentre` in `model/grid.ts` branch on the type and return axial
  coordinates on a hex grid, so `table-menu.ts`, `gestures.ts`, `ruler.ts` and the
  debug panel became hex-aware with no change of their own.

### Phase 2 — seeing it

- One grid program, one `u_type` uniform, one hex branch: world point to axial,
  round, distance to the nearest of three edge normals, same `fwidth()` edge, same
  dash, same zoom fade. Verified with `glslangValidator` and software-rendered to
  ASCII to confirm the honeycomb tiles and is the right size.
- `GLYPHS` is `"0123456789 ft.mikhexsq"`. `distanceLabel(value, grid)` replaces
  `distanceLabel(feet)`.
- `modes/ruler.ts` walks `cellPath` (supercover or `hexLine`) and counts
  `cellsBetween` (Chebyshev or cube distance). `lineRuler` delegates to `walkRuler`
  on a hex grid, so the measure tool counts hexes.

## What Checkpoint 2 landed — phase 3, the GM's map and the players' map

- `Layer.GMMap *MapRef` (`json:"gmMap"`). **The GM sees `GMMap` if it is set,
  otherwise `Map`. Players always see `Map`.** Every existing room keeps its single
  map as the players' map; nothing migrates and `room.Schema` stays at 3.
- `Project` clears `GMMap` on every layer for a non-GM role, which is one loop and
  covers both the reconnect snapshot (`sync.go`) and every derived change, because
  `Derive` diffs projections. `TestThePlayersAreNeverToldWhichMapTheGMIsLookingAt`
  asserts on the **marshalled bytes** of a player's projection, so a future field
  carrying the asset id back out by another route fails that test rather than a table.
- `TableSetLayerMap` and `TableClearLayerMap` gained `GM bool`; `TableClear` empties
  both slots; `cloneLayers` clones both; `SceneLoad.Resolve` re-reads both through the
  same `lib.Map`, clearing the slot and naming the floor when the library refuses.
- `render/layers.ts` picks `layer.gmMap ?? layer.map`. It needs no role check: the
  projection is what makes that correct, and the crossfade works between the two
  slots as it already did between floors.
- The layer manager renders two slots per floor through `RoomLayersData.Slots`,
  labelled *Players see* and *You see*; the GM's empty slot reads *Same as the
  players* rather than *No map*. `RoomLayer` now holds two `RoomLayerMap` values
  instead of a flattened `MapID`/`MapName`/`Width`/`Height`/`Mismatch` set, which is
  what lets one `layerMapSlot` templ component serve both.
- The picker's heading says which slot it is filling.

## What Checkpoint 3 landed — phase 4, fog by the cell

- `modes/cells.ts`: `newCellWalker()`, a gesture-scoped walker that turns pointer
  samples into the cells newly entered, using `cellPath` (supercover on a square grid,
  `hexLine` on a hex one) and a `Set` of what this gesture has already touched.
  **Phase 6's stamp brush uses this unchanged.**
- `FogOptions.cells`; the fog tool's press/drag/release paint through the walker and
  emit one `fog.add` per cell — a `rect` of the cell's bounds on a square grid, a
  `poly` of `hexCorners` on a hex one. The cell under the pointer is outlined through
  `overlay.cells`, which already draws hexagons.
- **Nothing in `internal/room` changed**, which is what proves rule 1: the fog model
  never learned what a hex is. `TestAHexagonOfFogIsAcceptedAndCostsItsTwelveCoordinates`
  is the guard, not a new feature.

**Fixed after the first hand-over: the brush preview sat half a cell out.**
`overlay.Cell.x/y` is the **top-left corner** — `path-pass.ts:cell` re-adds `size / 2`
to reach the centre — and the first write passed the centre. `ruler.ts` and `select.ts`
both subtract half a cell and are the precedent. The test that shipped with the bug
asserted the wrong numbers rather than catching it; it is replaced by
*the outlined cell is the one the brush sends*, which derives the sent shape's bounding
box and compares its centre with the outline's, for all three grid types. **Any future
producer of `overlay.cells` takes the corner, not the centre.**

**Fixed at the same hand-over: the brush outlived the fog tool.** `switch.ts` calls
`hover` and `contribute` on **every** tool each frame, not only the chosen one, so the
next mouse move after switching to Select put the pointer back on the fog tool and the
outline came back. Every other fog preview is gated on a gesture, which `settle()`
abandons on the way out; a hover-only preview needs the tool's own `enter()`/`leave()`
flag. `draw.ts` already carried exactly that for its brush cursor and is the precedent
`fog.ts` now follows. **Phase 6's stamp brush contributes a ghost from hover and needs
the same gate.**

## What Checkpoint 4 landed — phase 5, the terrain shelf

- **`terrain` is a third library kind, not a third shape of code.** `libraryKind` already
  bound the type, the slug, the storage key and the two templ components at registration,
  so `terrainKind` is one literal and five one-line handlers. Everything behind them —
  upload, replace, rename, delete, list, search, the card, the name box, the quota — is
  the code the tokens already ran.
- `storage.TerrainKey` is `users/<owner>/terrain/<asset>`, beside `TokenKey`.
  `images.Fit(src, 512)` keeps the picture whole; **it is never squared**, because the
  cell mask is applied at draw time in phase 6.
- `db/migrations/20260913120000_terrain_assets.sql` widens the assets type enum; down
  deletes the terrain rows and narrows it again. **Applied locally, and `db/schema.sql`
  is dbmate's own dump, so `make sqlc` already yields `queries.AssetsTypeTerrain`.**
  No new query: `GetLibraryAssets`, `SearchLibraryAssets`, `GetLibraryAsset` and
  `ReplaceLibraryAsset` all take the type as a parameter.
- `room.PictureTerrain` and a three-way `hub/library.go:assetType`. The stub in
  `internal/hub` now answers no rows when the bound type is not the row's own, so
  *a token is not served as terrain* is a real refusal rather than a hopeful assertion.
- The tab strip is five wide, *Terrain* second, after *Maps*. The picker prose and the
  empty state live in `asset-grid.go` beside the other four.

## What Checkpoint 5 landed — phase 6, tiles

- **A tile has no id.** `(LayerID, Q, R)` is its identity, so one-per-cell is a property
  of the data rather than a rule. `Palette []TileArt` sits on `Table` beside `Layers`
  (never inside `TableSettings`, which `Derive` compares with `!=`); `Tiles []Tile` sits
  on `State`. The image URL lives once per palette entry, not once per stamp.
- `cellDiff[T]` in `derive.go` beside `diff[T]`, keyed by `(layer, cell)`. It emits one
  `TilesErased{Layer, Cells}` **per floor** — `PaletteRemove` can take cells off several
  at once — and one `TilesStamped{Tiles}` for everything added or changed. Neither maps
  to a UI event in `panels.ts`; `palette.updated` maps to `ROOM_TABLETOP` the way
  `layers.updated` does. `revisions.ts` gained a `tiles` slice, bumped by both tile
  changes and by `palette.updated` (a changed bag changes what is drawn).
- Commands: `PaletteAdd` (a `Resolver` reading `lib.Picture(..., PictureTerrain)`, so a
  token id is refused), `PaletteRemove` (erases the tiles), `TilesStamp`, `TilesErase`,
  `TilesClear`. All GM-only until phase 7. Caps `TilesMax = 4_000`, `PaletteMax = 24`,
  `TileBatchMax = 64`, `CellLimit = 100_000` in `validate.go`.
- **`TilesStamp` counts the fresh cells before it lays any of them**, so a batch that
  would cross `TilesMax` is refused whole rather than half-applied. Cells are deduped on
  the way in, which is also what makes one batch naming a cell twice leave one tile.
- **Rendering is the pawn program with a fourth branch.** `SHAPE_HEX = 3` and
  `SHAPE_HEX_FLAT = 4` share one branch: `d = max(|x|, 0.5|x| + |y|) <= 1` over the
  quad's own local coordinates, with the axes swapped for a flat top. The quad is the
  cell's bounding box from `cellExtents`, so a pointy-top hex is `S` wide and `2S/√3`
  tall. `render/terrain-pass.ts` is a second batch on `resources.pawnProgram`;
  `stages/terrain.ts` sits between `tilesStage` and `gridStage` and draws the ghost from
  `overlay.stamps` in a second pass at half alpha.
- **Rotation is restricted to 60° on a hex grid and 90° on a square one because the mask
  turns with the art.** The vertex shader rotates the quad and the fragment tests
  `v_local`, so the hexagon rotates too — and a regular hexagon is invariant under 60°,
  a square under 90°. Any other angle would cut the art against a turned cell.
- The wheel grew a ring: **one ring carrying the actions and then the pictures**, each
  button `--degree` (`90 - i×360/n`, clockwise from the top) and `--radius`
  (`max(96px, n×48px/2π)`) from the server, numbered across both lists by
  `NewTableMenu`. One arbitrary-property class in the `.templ` moves them; the script
  never computes a position but the wheel's own. The centre holds a
  `data-table-menu-label` naming the picture under the pointer, and **the wheel opens
  centred on the cell it acts on, not on the pointer**.
- **There is no stamp brush.** A tile is placed one at a time, from the wheel: hover a
  picture and it previews on the cell the wheel was opened on, `[` and `]` turn the
  preview by the cell's own step, a click stamps it at that turn, and Escape closes.
  The turn outlives the wheel so a row goes down the same way up, and it is snapped to
  what the grid can do, so an angle set on a square grid is legal on a hex one. The
  preview reaches the overlay through `TableMarks`, the channel `select.ts` already
  used to draw the marked cell.
- The GM manages the bag in a *Tile palette* window: a compact strip of chips at the
  top, each with a *Remove* on hover behind a confirm that says the stamped cells go
  with it, and the terrain shelf beneath it, scrolling on its own, where a picture
  already in the bag is marked *In use* and offers no *Add*. **The bag and the shelf
  are two short lists that swap themselves**, `?part=bag` and `?part=shelf` off the one
  fragment route, so adding a picture never replaces the window under the pointer.

**The stamp brush was cut on 2026-09-13, at Kyle's call, after the first browser pass.**
The plan's phase 6 gives a drag-painting brush armed by Shift-clicking the wheel. It went
in, and it was wrong: the arming gesture was invisible, the brush was a second mode
layered over a menu that already knew which cell it was acting on, and laying terrain a
region at a time is not how a hex crawl is drawn. **`modes/stamp.ts` is deleted and the
wheel does the whole job.** `switch.ts` is back to one special-cased tool (`place`), and
`TileBatchMax` stays on the server as a wire cap with nothing on the client mirroring it.
**Phase 7's `TilesErase` ownership rules still apply** -- the wheel's *Erase* is the one
control they gate.

**Fixed at the first hand-over, from five reports in a browser:**

1. **A tile never appeared for anyone.** `terrainStage` rebuilt its batch only when its
   own `tiles`/`table` watch moved, but `sprites.sprite()` returns `null` until the
   picture has loaded — so the first build pushed nothing and no later frame ever built
   again. **A stage that reads the sprite cache must also rebuild on `frame.rebuild`,**
   which is the renderer's signal that `sprites.epoch()` moved. Worse than the missing
   tile: `resources.begin(rebuild)` clears the cache's `live` set on every rebuild
   frame, so a stage that skips those frames lets its own pictures be evicted.
   `stages/pawns.ts` had it right all along and is the precedent.

2. **The wheel had two rings.** The plan's hub-in-the-middle read as two unrelated
   controls. Everything now sits on one ring, actions first from the top so they keep
   their place as the bag grows, all at `btn-circle` size.

3. **The wheel opened under the pointer.** It now opens centred on the cell it acts on:
   `screen + (centre - map) / mapPerPixel`, which needed the scale the rest of the
   tools already take.

4. **The palette window was unreadable.** Two grids of large cards with no way to tell
   the bag from the shelf. The bag is now 40px chips in a capped strip and the shelf is
   a tighter grid marked *In use*, with the shelf scrolling inside the window rather
   than the window scrolling.

5. **Adding a picture threw the window to the bottom of its scroll**, because the whole
   fragment swapped itself on `room:tabletop`. Only the bag and the shelf refetch now,
   each replacing a short list in place.

## What Checkpoint 6 landed — phase 7, players stamp

- **`PlayersCanStamp` is a table option, off by default.** It sits on `TableSettings`
  beside `PlayersCanDraw`, travels in `TableSetOptions`, and is rendered in the *Table
  settings* window rather than the *Grid* one: it is how the GM runs the table, not what
  is on it, so it stays with the room across a scene load. The two forms are kept apart
  by `TestNeitherTableFormCarriesTheOthersFields`, and `scene_test.go` holds the setting
  across an import.
- **`TilesStamp.Authorize` is `StrokeBegin.Authorize` with a different noun.** The floor
  check first (`requirePlayerLayer` — a player acts on the floor the table is showing),
  then the setting, and the refusal reads *Stamping is off / The GM has turned off
  stamping for players.* `TestStampingOffIsRefusedTheWayDrawingOffIs` holds that
  sentence against the drawing one word for word, so the two cannot drift apart.
- **`TilesErase.Authorize` is `StrokeErase.Authorize`:** the GM erases anything, a
  player only cells whose tile carries their id, through a `requireOwnTile` shaped like
  `requireOwnStroke`. `Authorize` runs over the whole batch before `Apply` lays a
  finger on it, so one cell that is not theirs takes the whole erase down with it.
- **Anybody stamps over anybody.** A tile in a cell is replaced by whoever stamps next,
  GM or player, and the tile remembers who laid it last -- so stamping over somebody
  else's tile makes it yours to rub out. Erasing stays own-only. See departure 22.
- **The ring follows the setting, not the role.** `tableMenuRingFor(isGM, table)` in
  `internal/controllers/room-table-menu.go` is the whole policy: a player gets the GM's
  ring, the same entries in the same order, once the toggle is on, and nothing while it
  is off. The wheel already refetches on `ROOM_TABLETOP`, which `table.updated` raises,
  so flipping the toggle reaches a player's wheel with no reload.
- **The eraser is on the wheel whenever there is a ring**, GM or player; *Party starts
  here* is still the GM's alone, and so is the palette window — only their Tabletop menu
  lists it, and `RoomPaletteFragment` still goes through `gmTable`.
- **A scene carries its tiles and the bag that keys them** -- a CP5 gap found while
  building this checkpoint, and fixed here. `ExportScene` already cloned them into the body; it now clears
  `Tile.By` the way it clears a stroke's, so a loaded scene's tiles belong to nobody and
  only the GM can rub them out. `ImportScene` copies `Tiles` and `Table.Palette` beside
  the layers, and `SceneLoad.Resolve` re-reads every palette picture out of the library
  the way it re-reads a map ref: a renamed or reprocessed picture arrives current, one
  that has been deleted is dropped along with its tiles and named in `Missing`, and a
  library that is *broken* rather than missing still fails the whole load.
- **A scene's bag replaces the room's, it does not merge with it.** See departure 25.
- **Nothing on the client changed but `store.ts`'s `empty()`.** The wheel is
  server-rendered per role and `table-menu.ts` has never known what a role is, so a
  player's wheel is the GM's code reading the GM's markup with the party start left out.

## What Checkpoint 7 landed — phase 8, the hex key

- **A note has no id either.** `HexNote{LayerID, Q, R, Title, Body, Revealed}` is keyed by
  its cell exactly as a tile is, sorted by the same `compareCells` and diffed by the same
  `cellDiff[T]` into `notes.upserted` and one `notes.removed` per floor. Commands
  `NoteSet`, `NoteReveal` and `NoteRemove`.
- **Everybody writes the key; the GM decides what is shared and what is rubbed out.**
  `NoteSet` takes a player on the floor the table is showing, and **a note a player
  writes is shared from the moment it exists** -- an unshared note is invisible to them,
  so writing one they could not then read would be a hole in the floor. `NoteReveal` and
  `NoteRemove` stay the GM's. A hex already holding a note the GM is keeping refuses a
  player's write: *The GM is keeping that hex to themselves.*
- **A player's copy holds the shared hexes and nothing else.** `Project` drops every
  unrevealed note rather than blanking it -- departure 27 -- so revealing derives as
  `notes.upserted` to the players and taking it back derives as `notes.removed`, which
  is also what shuts a player's open window. A hex the GM is keeping opens for a player
  as an **empty editor**, so opening it discloses nothing; only a write collides.
- `NotesMax = 500`, `NoteBodyLimit = 4_000` and `NoteBytesBudget = 200_000` in
  `validate.go`, with `noteBudget` shaped like `fogBudget`. A note with neither a title
  nor a body is refused: an empty note is a hex that looks written on for nothing.
- **`GET /fragment/room/hex?room=&layer=&q=&r=`** answers everybody with the same editor;
  the GM's additionally carries *Shared with the party* and *Rub out*. The window id is
  `hex:{layer}:{q}:{r}`, the way a stat block's is suffixed.
- **The hex key is opened by double-clicking any hex**, the gesture that opens a pawn,
  and by *Hex note* on the wheel, which everybody now has. **Nothing is drawn on the map
  for a hex that has a note** -- departures 28 and 31.
- **The editor never re-renders under anybody's hands.** A save swaps only
  `#room-note-actions`, so a first save on an empty hex unlocks the GM's two controls
  without taking the caret out of the textarea, and nothing refetches a note that is
  open. Last writer wins on a hex two people are editing at once.
- **Erasing a cell rubs out what is written on it.** The wheel's *Erase* and *Clear
  tiles* take the notes on those cells with the tiles, for the GM. A player's erase takes
  their tile and leaves the writing, because rubbing out a note is the GM's however it is
  rubbed out.
- **A scene carries the hex key**, including which hexes the party had already been told
  about, the same way fog carries its reveals.

## What Checkpoint 8 landed — phase 9, numbering the cells

- **`Grid.Numbered bool`** (`json:"numbered"`), a *Number the cells* toggle in the *Grid*
  window under the colour. It sits in `Grid`, so it travels with the scene: the numbering
  belongs to the map, not to the viewer. Nothing migrates and `room.Schema` stays at 3.
- **The map is what starts and stops the count.** Only cells whose centre sits over the
  floor's picture are numbered, and a floor with no map is not numbered at all. The
  players' map is preferred over the GM's keyed one, so flipping to a keyed map of a
  different size does not renumber the table.
- **1 is the top-left cell; the count runs down a column and then into the next.**
  Columns come from offset coordinates -- odd-r for point-up hexes, odd-q for flat-top --
  so a column on a point-up grid is the zigzag every paper hex crawl uses, and the same
  code numbers a square grid.
- **`model/numbering.ts`** builds the numbering once per grid and map and holds the last
  one. `of(q, r)` is a lookup into an `Int32Array`; `each(rect)` walks only what is on
  screen. A 1024 by 768 map at 64px runs to 231 cells on a point-up grid, 225 flat-top,
  192 square.
- **`render/stages/numbers.ts`** is a second `createPathPass`, like `floor-marks` and
  `over-marks`. `PathPass` grew `caption`, which is `label` at a world size rather than a
  fixed thirteen pixels and with no outline behind it; `label` keeps its halo through
  `haloed`. The numbers fade out between eleven and six pixels tall, the way the grid
  lines fade.
- **They are drawn flat, in the grid's colour**, alpha and all.
  **No new shader, no new atlas, no new pass** -- `GLYPHS` already held the digits and
  `visibleRect` already culled.
- **The GM sees them and players do not.** The stage is in the GM's order alone, between
  the fog and `floor-marks`, so the fog does not dim it and a pawn stands over it.
- **The hex window is titled by the number** when the table is numbered, so
  double-clicking a hex names it the way the GM's notes do. A player's window still reads
  `Hex 3, -2`, because a player has no numbers to match it against.

## Departures from the plan, and why

1. **The pre-field snapshot fixture is new.** The plan asks phase 0 to assert against
   `testdata/snapshots/schema-3.json`, but `TestTheCurrentSchemaHasAGoldenSnapshot`
   regenerates that file under `go test -update`, so it cannot stand for a snapshot
   written before the field. Added
   `testdata/snapshots/grid-before-type.json`, hand-written without `type` or `units`.
   **Every future phase in this plan should use that file, not the golden.**

2. **`overlay.cells` grew a `type`, and the path pass fills a hexagon.** The plan
   never mentions it, but `Cell` was `{x, y, size}` and `floor-marks` drew a square —
   so the ruler's crossed cells, the wheel's marked cell and the party-start marker
   were all squares over hexes. A regular hexagon is exactly three rhombi, so
   `pass.cell` emits three parallelograms and needs no new pass and no shader.
   **Phase 8's note markers get this for free, and CP3's brush cursor did.**

3. **The hex-hiding class must be a literal in the `.templ`.** It was first written as
   a Go const in `room-grid.go` and Tailwind silently dropped it — `@source` is
   `../templ/**/*.templ` and nothing else. The literal
   `group-has-[input[value^=hex]:checked]:hidden` now sits in `room-grid.templ`, and
   `TestTheGridFormHidesDiagonalsOnAHexGridWithoutAskingTheServer` asserts on that
   string. Confirmed emitted into `public/css/app.css`.

4. **`roundAway` and `hexRound` normalise negative zero.** JavaScript produces `-0`
   where Go's `int(math.Round(...))` produces `0`. Left alone it would have been a
   silent cross-language disagreement in exactly the place the plan warns about.

5. **`PathCellsMax` is now a Go constant** with
   `TestThePathCapMatchesTheServers` reading it back out of `hex.ts`, in the idiom
   `SELECTION_MAX` and `OBJECT_PIXELS_MAX` already use. It had been TypeScript-only.

6. **`lineRuler` delegates to `walkRuler` on a hex grid.** The plan says measure and
   ruler both count hexes; measure used the free euclidean path, so on a hex grid it
   now takes the cell path instead. Square grids keep the free ruler.

7. **`TableClearLayerMap` gained `GM bool` too.** The plan only names
   `TableSetLayerMap`, but a GM who cannot clear the keyed map has to overwrite it.

8. **The scene thumbnail falls back to the GM's map.** `hub/actor.go:export` read
   `l.Map` alone, so a floor mapped only for the GM would have saved a scene with no
   preview. It prefers `Map` and falls back to `GMMap`. The scene list is the GM's
   own surface, so this leaks nothing.

9. **`GetImage` had to learn the new type, and an existing test said so.**
   `TestGetImageServesMonsterImagesAndNotJournalOnes` reads the enum out of
   `queries/models.go` and demands every member but `journal` and `music` appear in
   `GetImage`'s `type IN (...)`. Without it every terrain card's `<img>` would have
   404ed. **Any future asset type must be added to `sql/assets.sql:GetImage` or
   withheld there on purpose.**

10. **`noMatchHeading` grew a sibling that takes the verb.** Terrain is a mass noun, so
    the shared builder would have said *No terrain match "pine"*. `noMatchFor(subject,
    verb, query)` is the general form and `noMatchHeading` is now one line of it; only
    terrain passes `matches`.

11. **The hub has no *Hex note*.** The plan's phase 6 test names it, but the hex key is
    phase 8 and a button that does nothing is worse than no button. The hub is *Erase*
    and *Party starts here*; phase 8 adds the third and `TableMenuActions` is where it
    goes.

12. **`SHAPE_HEX` is two constants, not one.** A flat-top hexagon is a pointy-top one
    turned 90°, and turning the quad would have turned the art with it. `SHAPE_HEX = 3`
    and `SHAPE_HEX_FLAT = 4` share a single branch that swaps the axes.

13. **`overlay.stamp` is `overlay.stamps`, an array.** Every other overlay channel is a
    list and the render pass wants a list; one ghost is a list of one.

14. **`TableClear` empties the tiles and keeps the palette.** Tiles are on the board and
    go with the rest; the bag is a library the GM filled, and re-adding 24 pictures
    after every clear would be a punishment. *Clear tiles* is the separate, layered
    menu item the plan asks for.

15. **The eraser's ghost is an `overlay.cells` outline, not a picture**, because it has
    no picture to ghost. It follows CP3's rule and takes the corner, not the centre.

16. **There is no stamp brush at all** -- see the note above the departures. CP3 warned
    that a hover-only ghost needs a gate; the wheel's preview has one, because it only
    exists while the wheel is open and the pointer is on a picture.

17. **The wheel owns the stamp rotation**, not a tool. `[` and `]` are handled by
    `table-menu.ts`'s own keydown while the wheel is open, and ignored while it is
    shut.

18. **The wheel is one ring, not a hub and a ring**, and it centres on the cell rather
    than the pointer. Both were reported in the first browser pass; see the fixes above.

19. **The wheel writes no words over the table.** It is pictures and icons: the ring
    carries the art, the hub icons carry `data-tip` tooltips, and every button carries
    an `aria-label` so a screen reader still gets the name. A label in the middle of the
    wheel naming what was under the pointer was tried and cut -- over a map it reads as
    debug text, and the preview already says what the picture is.
    `TestTheWheelIsPicturesAndIconsAndNoWordsOverTheTable` is the guard.

20. **The palette fragment serves three parts off one route** (`?part=bag`,
    `?part=shelf`, or the whole window). A window that replaces itself on every table
    change throws away the reader's place in it, and the palette is the first window
    long enough for that to matter.

21. **Erasing is ownership, not permission.** `TilesErase` never consults
    `PlayersCanStamp`, exactly as `StrokeErase` never consults `PlayersCanDraw`. The
    wheel takes *Erase* away along with the ring when the GM turns stamping off, so in
    the browser the difference is only a command already in flight -- but the rule keeps
    the shape the drawing rules have, and a player is never told a tile they laid is
    somebody else's.

22. **Anybody may stamp over anybody's tile.** It was built the other way first -- a
    player could only replace their own -- on the reasoning that replacing a tile is
    erasing it by another name. Cut at the table's request: a shared map is stamped over
    by whoever is looking at it, and the GM has *Clear tiles* if it gets out of hand.
    Erasing is still own-only, so what a player cannot do is take a cell *away*; they
    can take it *over*, and `By` moves to them when they do.

23. **A player rubbing at an empty cell is told the tile is gone** (`CodeNotFound`),
    where the GM's erase treats an empty cell as a no-op. That is `requireOwnStroke`'s
    own shape: there is nothing there to own.

24. **The palette fragment answers a player 404, not the 403 the plan names.** Every
    GM-only fragment goes through `gmTable`, which is a bare 404 with an empty body,
    which is what CLAUDE.md asks a fragment's refusal to look like;
    `TestOnlyTheGMSeesTheTilePalette` is the guard. The palette *mutations* do answer
    403 -- `POST /rooms/{id}/palette` by a player is
    `TestOnlyTheGMFillsTheTilePalette` -- because they are not fragments.

25. **A scene's palette replaces the room's rather than merging into it.** Everything
    else a scene carries is replaced -- layers, grid, fog, drawing, pawns -- and a
    merged bag is both harder to explain and easy to overflow: `PaletteMax` is 24, and
    a GM cycling between two scenes would collect both their sets. The cost is real and
    visible in the convergence fixture: **opening a scene that has no terrain in it
    empties the bag.** The pictures are still on the terrain shelf, two clicks each to
    put back, and saving a scene captures the bag as it stood.

26. **The *scene is open* toast no longer blames the maps.** It read *without these
    maps*; terrain can be missing too, so it now reads *but not all of it could be
    read*, followed by the same list. `TestTheOpenedToastNamesWhateverCouldNotBeRead`.

27. **An unrevealed note is dropped from a player's copy, not blanked.** The plan asks
    for a note that projects with no title and no body so the marker still crosses.
    Blanking leaves a row saying *this hex has something in it* in a payload the player
    can read, and the plan's own sentence is that players learn a hex has something in it
    only once it is revealed. Hidden pawns are dropped rather than blanked for the same
    reason, so notes follow them.
    `TestAPlayerReadsOnlyTheHexesTheGMHasShared` still asserts on the marshalled bytes.

28. **Double-clicking any hex opens it, and *Hex note* is on everybody's wheel.** The
    wheel is fetched once per room and then shown at whatever cell was right-clicked, so
    an item that depends on *this* cell cannot be rendered server-side -- but it does not
    need to be, because every hex is worth opening now that everybody can write. The
    double-click is the gesture that already opens a pawn; a single click still just
    deselects.

29. **The reveal toggle and the rub-out live in their own swappable block.** A first save
    on an empty hex has to unlock them, and re-rendering the whole editor would take the
    caret out of the textarea mid-sentence. `POST /rooms/{id}/notes` answers with
    `RoomNoteActions` and the form targets `#room-note-actions` -- the palette's
    `?part=` lesson in a smaller place. The GM's editor deliberately does **not** listen
    for `room:notes`; a player's read view does.

30. **`paletteCommand` is now `roomCommand`, over a new `dispatchRoom`.** Three note
    routes wanted the same body, and one of them wanted to render afterwards instead of
    answering 204. `checkCell` came out of `checkedCells` for the same reason.

31. **A hex with a note is not marked on the map.** It was tinted gold at first --
    brighter once shared, fainter while the GM kept it -- and cut at the table's request:
    a wash over a stamped tile fights the terrain art it is painted on. The cost is that
    a hex keeps no sign of what is written on it, and the only way to find out is to
    open it; nothing is lost by opening one, because every hex opens.

32. **A hex note is plain text, not markdown.** The plan asks for the note to be rendered
    through `internal/markdown` for the player's read view -- but once everybody can
    write, there is no read view left to render into. The editor is what everybody gets,
    and the body is the text as typed. Say the word and the rendered view comes back as
    a second mode of the same fragment.

33. **The key's own UI event went away with the read view.** `room:notes` was added and
    then removed in the same checkpoint: the only fragment that listened for it was the
    player's read view, and an editor that refetches under a writer's hands is worse than
    one that goes stale. `notes.removed` still closes an open window through
    `window:close`, and a snapshot still reconciles a player's open hexes.

34. **The numbers are the grid's colour, drawn flat with no outline.** They were white
    with the ruler's near-black halo at first; both went on 2026-09-13, on the GM's say-so.
    `inkFor` reads the grid colour and nothing else, and `PathPass.caption` lays one run
    of glyphs where `label` lays nine. `label` keeps its halo through `haloed`, so the
    ruler is untouched.
    **The numbers take the grid's alpha as well as its hue**, so a faint grid is faint
    numbers, and a grid colour at zero alpha draws none at all -- the same early return
    the grid pass makes.
    **Nothing separates a number from what it sits on now**, so a number over busy terrain
    art is as readable as the colour the GM picked makes it. That is the trade the flat
    look buys.

35. **A hex whose centre lands exactly on the map's edge is numbered.** The rule is
    *centre over the picture*, which is the only rule that can be said in one sentence.
    On a point-up grid with the default offset of zero, that puts a half-column of
    half-hexes at the very left edge: with a 1024 by 768 map at 64px, cells 1 to 7 run
    down seven hexes that are half off the picture, and 8 starts the first full column.
    Any other grid offset, which is what aligning a real map gives you, does not do this.
    Worth an eyeball in the browser; a quarter-cell inset is a one-line change if it
    reads badly.

36. **The party pawn's reveal was cut, not deferred.** Everything phase 9 used to ask
    for -- `Pawn.Party`, `PawnSetParty`, `TableSettings.AutoReveal`, the reveal inside
    `PawnMove.Apply` -- is out of `plans/hex-crawl.md` as well as unbuilt. The scenes
    plan's note about `Pawn.Party` being lost on export went with it.

## Geometry, settled

- **`CellSize` is the across-flats dimension**, which is the pitch along the axis
  where hexes pack without offset. A 64px pointy-top hex is 64 wide and ~74 tall; a
  64px flat-top is ~74 wide and 64 tall. That is what makes a hex occupy the footprint
  the square of the same cell size did.
- **Hex `(0, 0)` centres where square cell `(0, 0)` centres**, at
  `(offsetX + S/2, offsetY + S/2)`, so a grid lined up against a map stays roughly
  lined up when the type changes.
- **The shader wraps its origin by the lattice period**, `S` one way and `S√3` the
  other, not by one cell. Wrapping by one cell would have broken the honeycomb.

## Carried forward

- **Freehand fog still snaps to square vertices on a hex grid.** `modes/fog.ts` uses
  `snapAxis` directly for the rectangle and the polygon. The cell brush is the
  hex-aware path, and it is the one to reach for on a hex grid. Left as the plan
  leaves it.
- **The cell brush is not the default on a hex grid.** Rectangle still is. Worth
  revisiting once a hex crawl has actually been run at a table.
- **`Pawn.Party` (phase 9) and the scene's existing *Party starts here* point are two
  different things sharing a word.** Both stay. The pawn context menu item needs
  wording that does not read as the same control.
- **The snapping hints still say "squares"** (*"A creature stands in the middle of the
  squares it fills"*). Only Diagonals was in scope to hide. Worth a pass later.
- **A GM map of a different size from the players' map is warned about, not refused.**
  The layer manager's size warning now checks both slots against the first map it
  finds, so a mismatched keyed map says *A different size from the other maps, so the
  grid will not line up*.

## Verifying Checkpoint 1

`make run`, then:

1. **An existing room is unchanged.** Grid, snapping and ruler behave as before. The
   *Grid* window now reads *Distance per cell* with a **Feet** dropdown beside it.
2. **The form.** *Grid type* radios at the top. Picking either hex option makes
   *Diagonals* vanish instantly, with no flicker and no refetch. Squares brings it back.
3. **The hexes.** *Hexes, point up* gives flat left and right edges with points up and
   down; *flat top* the opposite. A 64px hex occupies the width a 64px square did, so
   switching type should not visibly resize the table. Try dashed lines and zoom out to
   watch the fade.
4. **Offsets and cell size** slide and scale the honeycomb cleanly.
5. **Snapping.** A Medium creature lands dead centre in a hex — and so do Large and
   Gargantuan, straddling nothing. *Centre and corners* behaves identically to
   *Centre only*. *No snapping* drops where you let go. Objects are always free.
   **Watch for a pixel jump when you release a drag** — that is the two
   implementations disagreeing, and it is the failure this checkpoint guards against.
6. **Measuring.** The ruler highlights crossed cells as hexagons and counts hexes. The
   measure tool (`M`) counts hexes on a hex grid and still measures freely on a square one.
7. **Units.** Miles reads `12 mi`, Kilometres `12 km`, Cells `3 hex` on a hex grid and
   `3 sq.` on a square one. Under Cells the distance-per-cell number stops multiplying.
8. **The wheel.** Right-clicking empty ground outlines a hexagon, and *Party starts
   here* marks one.
9. **Reload**, and rejoin as a player in a second browser: same grid both sides.

## Verifying Checkpoint 2

Two browsers: you as GM, a second signed-in account as a player in the same room.

1. **Every existing room is unchanged.** Each floor now shows two rows where it showed
   one: *Players see* with the map that was already there, and *You see* reading
   *Same as the players*.
2. **Set a second map on *You see*.** The picker is headed *Choose the map you see*.
   The moment you pick it, your tabletop crossfades to it and **the player's does not
   move at all**. Pawns, fog, drawings and the grid stay exactly where they were on
   both screens, because there is one set of coordinates and two pictures.
3. **The player never learns the id.** With the player's devtools open on the Network
   tab, nothing under `/assets/maps/<your map's id>/tiles/...` is ever requested, and
   the socket frames carry `"gmMap":null`. This is the leak the whole phase exists to
   prevent.
4. **Clear *You see*.** Your view crossfades back to the players' map. Clearing
   *Players see* instead leaves your own map up and blanks theirs.
5. **A floor with only a GM map.** Clear *Players see* and leave *You see* set: you
   see your map, the player sees bare grid, and neither client errors.
6. **Sizes.** Put a differently sized map in the second slot; the row warns that the
   grid will not line up. It is a warning, not a refusal.
7. **Scenes.** Save a scene with both slots filled, clear the tabletop, reopen it:
   both maps come back, and the scene's thumbnail is there.
8. **Reload both browsers.** The split survives, because it is in the snapshot and the
   projection.

## Verifying Checkpoint 3

1. **The fog options pill has a third button**, a grid icon, beside Rectangle and
   Polygon. Only one of the three is ever lit.
2. **On a square grid**, with *Cells* and *Uncover* chosen, drag across the table:
   each cell you touch clears as you enter it, aligned to the grid, and the cell under
   the pointer is outlined while you work. Drag back over a cell you already cleared —
   nothing happens twice.
3. **Move fast.** Whip the pointer across the table in one flick: every cell on the
   line clears, not just the two the pointer was sampled at.
4. **On a hex grid**, the same drag clears hexagons that tile perfectly against each
   other, with no square corners poking out.
5. **Cover works too.** Switch to *Cover* and paint the fog back one cell at a time.
6. **Ctrl+Z** still takes back the last shape — one cell per press, since each cell is
   its own shape.
7. **Rectangle and Polygon still work**, including on a hex grid, where they still
   snap to square vertices. That is expected; the cell brush is the hex-aware one.
8. **The player sees it land** as it is painted, and it survives a reload.

## Verifying Checkpoint 4

`make run` applies the migration on the way up. Then, in the Asset Manager:

1. **The strip is five tabs**, *Maps · Terrain · Tokens · Avatars · Music*, in that
   order, and every page reaches every other. Exactly one is marked current.
2. **The empty state.** *Terrain* on a fresh account reads *No terrain yet.* with its
   own blurb, not the tokens' one.
3. **Upload.** *Upload terrain* takes a picture. The card appears at the top of the
   grid with a toast, shows the image, and reads its stored size — **not squared.**
   Upload a wide picture and a tall one and confirm each keeps its aspect, the long
   side at 512.
4. **The card's controls.** Rename in place, replace the file, and delete behind the
   confirm. All three act on `/assets/terrain/<id>`.
5. **Search.** Type into the box: the grid filters as you type. A term that matches
   nothing reads *No terrain matches "…"* — **matches, not match** — with the hint
   under it.
6. **The kinds do not leak into each other.** A terrain picture is not listed under
   *Tokens*, and a token is not listed under *Terrain*. Take a terrain id and try it in
   a token URL — `PATCH /assets/tokens/<terrain id>/name` — and it is a 404, not a
   quiet rename.
7. **The image route serves it.** The card's thumbnail is `/assets/images/<id>`; if it
   renders, `GetImage` has the new type. A broken image here is departure 9 regressing.
8. **Reload**, and the pictures are still there. Nothing in the room changed this
   checkpoint — the shelf is stocked, and phase 6 is what spends it.

## Verifying Checkpoint 5

Upload two or three terrain pictures under *Assets → Terrain* first; the palette has
nothing to offer until you do.

1. **The palette window.** *Tabletop → Tile palette*. An empty chip strip at the top
   with a count reading *0 of 24*, and the terrain shelf below it. **Only the shelf
   scrolls**; the window itself does not. Type in the search box: the shelf filters and
   nothing else on the window moves.
2. **Add two pictures.** *Add* on a shelf card; it becomes *In use* and a chip appears
   in the strip above. **The window does not jump** — you stay exactly where you were
   in the shelf. Adding the same picture twice is not offered. Hovering a chip in the
   strip reveals its *Remove*.
3. **The ring.** Right-click empty ground. **The wheel opens centred on the cell, not
   on the pointer** — click near a cell's edge and it still lands on the middle of that
   cell. *Erase* and *Party starts here* sit at the top of **one ring** with your
   pictures following them round; the actions stay put as you add pictures. Hover a
   picture and it previews in the cell rather than naming itself -- **nothing on the
   wheel writes text over the table**. With one picture the ring is close in; with
   twenty-four it is pushed out far enough that none of them overlap.
4. **Stamp one cell.** Pick a picture. The wheel closes and that cell fills with the
   art, clipped to the cell — square on a square grid, a hexagon on a hex one, tiling
   against its neighbours with no square corners. **The tile sits under the grid lines,
   under the drawing and under the fog, and over the map.**
5. **Stamp over it.** Right-click the same cell, pick the other picture: it replaces the
   first. There is never more than one tile in a cell.
6. **The preview.** Right-click a cell and **hover** a picture without clicking: a
   half-transparent copy of it appears in that cell. Move to another picture and the
   preview follows; move off the ring and it goes.
7. **Turning it.** While hovering, press `]` and `[`. The preview turns -- by 90 degrees
   on a square grid and 60 on a hex one -- and clicking lays it at that turn. **The cell
   never changes shape**, only the picture inside it. The turn is remembered for the
   next cell, so a row goes down the same way up.
8. **The eraser.** *Erase* takes the one cell the wheel was opened on.
9. **Escape** closes the wheel and takes the preview with it. So does clicking away,
   scrolling, or picking anything.
10. **There is no brush and no dragging.** One right-click lays one tile.
11. **The players see it.** In a second browser as a player, every tile appears as it is
    painted, under their fog. A player right-clicking empty ground gets **no ring and no
    eraser** — phase 7 is what opens those to them. (From CP7 their wheel does carry
    *Hex note*.)
12. **Remove a picture from the bag.** The confirm says the stamped cells go with it.
    They do, on every floor, and the ring loses that button.
13. **Floors.** Stamp on the ground floor, switch floors, stamp there: each floor keeps
    its own tiles. *Tabletop → Clear tiles* empties the floor you are looking at and
    leaves the other alone, and leaves the bag full.
14. **Reload both browsers**, and everything is still there. Save a scene with tiles on
    it and clear the tabletop: the tiles go and **the palette stays**. Reopen the scene
    and both come back — that is the CP6 scene fix, and step 12 of the CP6 plan drives
    it properly.

## Verifying Checkpoint 6

Two browsers, the GM in one and a player in the other, at a table with a few pictures
in the bag and a tile or two already stamped.

1. **The toggle.** *Tabletop → Table settings* carries *Players may stamp terrain*
   under *Players may draw on the tabletop*, **off**, with its own line of prose. The
   *Grid* window does not carry it, and saving one form does not disturb the other.
2. **While it is off**, the player right-clicks empty ground and **nothing opens** --
   the same as at the end of CP5.
3. **Turn it on.** Without either browser reloading, the player right-clicks again: the
   wheel carries **your ring, in your order**, and *Erase*, beside the *Hex note* they
   always had — and **no *Party starts here***.
4. **The player stamps.** Hovering a picture previews it in the cell, `[` and `]` turn
   the preview, clicking lays it. It appears on your table at once, and on any other
   player's.
5. **The player takes their own back.** *Erase* over the cell they just laid: gone.
6. **The player cannot erase yours.** *Erase* over a cell you stamped is refused with
   *Not your tile*, and the tile stays put.
7. **But they can stamp over yours.** Picking a picture over a cell you stamped
   replaces it, turn and all — and the cell is now theirs, so they can rub it out and
   you are the one who cannot. Whoever stamped last owns the cell.
8. **You are unchanged.** Erase their tile, stamp over it, *Clear tiles* -- all still
   yours, however the toggle sits.
9. **The bag is yours alone.** The player's *Tabletop* menu has no *Tile palette*, and
   `/fragment/room/palette?room=…` typed into their address bar is an empty 404.
10. **Turn it off again.** On the player's next right-click the ring and the eraser are
    gone together, leaving *Hex note* alone on the wheel; the tiles they laid stay on
    the table.
11. **Reload both.** The setting survives, and so does who laid what: the player can
    still erase their own and still cannot erase yours.
12. **Scenes carry the tiles now.** Stamp a few, save a scene, *Clear tabletop*, then
    reopen the scene: **the tiles come back and so does the bag**, on the right floors,
    at the right turns. The reopened tiles belong to nobody, so a player cannot rub them
    out and you can.
13. **A scene with no terrain empties the bag.** Open an older scene saved before any of
    this: your palette goes with it, because a scene's bag replaces the room's. The
    pictures are still on the *Terrain* shelf. Say so if that is the wrong call —
    departure 25 is where the reasoning is.
14. **Terrain that has been deleted.** Save a scene, delete one of its terrain pictures
    from *Assets → Terrain*, then reopen the scene: it opens with a toast naming the
    picture, its cells are empty rather than blank-but-stamped, and the rest of the
    scene is fine.
15. **The setting is the room's.** Open another scene and come back: *Players may stamp
    terrain* has not moved.

## Verifying Checkpoint 7

Two browsers again, GM and player, on a floor with a hex grid and a few tiles stamped.

1. **Write one.** Right-click a hex: the wheel has a third action, the book. Pick it and
   a *Hex q, r* window opens with a title and a body. Write something and click away:
   **the window does not jump or lose what you typed**, and *Shared with the party* and
   *Rub out* appear under it.
2. **Double-click instead.** Double-click any hex — written on or not — and the same
   window opens. A single click still just deselects, and double-clicking two different
   hexes in a row opens neither.
3. **Nothing is drawn on the map.** A hex with a note looks exactly like one without,
   over terrain and over bare grid alike.
4. **The player cannot read it yet.** They open the same hex: an empty editor, with no
   sign that you have written anything there.
5. **Share it.** Flip *Shared with the party*. The player reopens the hex and your words
   are in it.
6. **They write.** The player types into a hex of their own and saves: it appears on your
   table, already shared — their editor has **no share toggle and no *Rub out***.
7. **They edit yours.** With the hex shared, the player changes the body: your next open
   shows their words. Both of you writing the same hex at once is last-writer-wins.
8. **They cannot take one of yours.** Write a hex and leave it unshared, then have the
   player write on that same hex: refused with *The GM is keeping that hex to
   themselves*, and what you wrote is untouched.
9. **Take one back.** Turn sharing off on a hex the player has open: **their window
   closes itself**.
10. **Rub it out.** *Rub out* asks first, then closes the window everywhere.
11. **The eraser takes the writing.** Stamp a tile on a hex with a note, then *Erase*
    that cell from the wheel: the tile and the note both go. *Clear tiles* empties the
    floor's notes with its tiles. A player erasing their own tile leaves the note alone.
12. **Floors.** A note on the ground floor and one in the cellar stay apart. Deleting a
    floor takes its notes.
13. **Scenes.** Save a scene with a written and a shared hex on it, *Clear tabletop* —
    the notes go — then reopen: **both come back, and the shared one is still shared.**
14. **Reload both browsers.** Everything is where it was.
15. **The long one.** Four thousand characters in a body is taken; more is refused with a
    plain message rather than a silent truncation.

## Verifying Checkpoint 8

One browser as the GM, one as a player, on a floor whose map is loaded.

1. **The toggle.** *Tabletop -> Grid* carries *Number the cells* under the colour, off.
   Turn it on: numbers appear over the map without a reload, one per cell, anchored at
   the top of each.
2. **The count.** 1 is the top-left cell. Counting runs **down** the leftmost column,
   and the next number after the bottom one is at the top of the next column to the
   right.
3. **The map is the edge.** Nothing outside the picture is numbered, on any side. Pan
   off the map: no numbers out there, however far you go.
4. **Both hex types and squares.** Switch *Grid type* between the three: the numbering
   follows and stays readable. On *Hexes, point up* a column zigzags left and right as it
   descends, which is what a paper hex crawl does; on *flat top* it runs straight down.
5. **The left edge on a point-up grid.** With *Offset across* at 0 the first few numbers
   sit on half-hexes hanging off the left edge (departure 35). Nudge *Offset across* by
   half a cell and they go. Say if the default should hide them instead.
6. **Zoom.** Zoom out: the numbers shrink with the hexes and fade out rather than piling
   into mush. Zoom back in: they come back. Zoom right in: they stay at the top of each
   cell, not the middle.
7. **The colour follows the grid.** Change *Colour*: the numbers change with it. Drop the
   colour's alpha and the numbers go as faint as the lines; take it to zero and they go
   altogether. There is no outline behind them, so check the colour you run with reads
   over your darkest map, your lightest one and a stamped tile.
8. **Cell size and offsets.** Change *Cell size* and both offsets: the numbering rebuilds
   to match, and the count starts again from the new top-left cell.
9. **The player sees nothing.** With numbering on, the player's table has no numbers
   anywhere, and their *Tabletop* menu has no *Grid* window to turn it on with.
10. **The hex key knows the number.** Double-click a numbered hex: the window is titled
    *Hex 47* rather than *Hex 3, -2*. The player double-clicking the same hex gets
    *Hex 3, -2*. Turn numbering off and the GM's next open reads *Hex 3, -2* too.
11. **A floor with no map.** Switch to a floor with no picture on it: no numbers, and no
    error.
12. **Two floors, two maps.** Each floor numbers its own map from its own top-left, and
    the layer bar moving you between them renumbers cleanly.
13. **The GM's own map.** Put a keyed map of a different size in *You see* on the same
    floor: the numbers do **not** move, because they are anchored to the players' map.
14. **Scenes.** Save a scene with numbering on, *Clear tabletop*, reopen it: numbering
    comes back on with the map, and the numbers are the same ones.
15. **Reload.** The toggle and the numbers survive.

## Files

**Checkpoint 1.** Added: `server/internal/room/hex.go`, `hex_test.go`,
`testdata/snapshots/grid-before-type.json`, `server/js/room/model/hex.ts`,
`hex.test.ts`, `server/js/room/render/glyphs.test.ts`, `grid-pass.test.ts`.
Changed: `internal/room/{state,snap,validate}.go` and their tests,
`internal/controllers/room-table.go` and its tests, `templ/pages/room-grid.{go,templ}`,
`templ/pages/room-table_test.go`, and on the client `model/{grid,shape,stroke,overlay}.ts`,
`modes/{ruler,select}.ts`, `render/{grid-pass,path-pass,glyphs}.ts`,
`render/shaders/grid.ts`, `render/stages/floor-marks.ts`, `store.ts`, plus every grid
literal in the test helpers.

**Checkpoint 2.** Changed: `internal/room/{state,table,project,scene,command}.go`,
`internal/room/{projection,derive,layer,scene_load,fog}_test.go`,
`internal/hub/actor.go`, `internal/controllers/room-table.go` and its tests,
`routes.go`, `templ/pages/{room-layers.go,room-layers.templ,room-maps.go,room-maps.templ}`,
`templ/pages/room-table_test.go`, and on the client `render/layers.{ts,test.ts}` plus
every `Layer` literal in the test helpers.

**Checkpoint 3.** Added: `server/js/room/modes/cells.ts`, `cells.test.ts`.
Changed: `js/room/modes/{fog.ts,fog.test.ts,testing.ts}`, `js/room/fog-tool.ts`,
`templ/pages/{room.go,room.templ,icons.templ}`, `templ/pages/room-fog_test.go`,
`internal/room/fog_test.go`.

**Checkpoint 4.** Added: `db/migrations/20260913120000_terrain_assets.sql`.
Changed: `db/schema.sql`, `server/sql/assets.sql`,
`internal/controllers/{library-assets.go,library-assets_test.go,asset-search.go}`,
`internal/storage/keys.go`, `internal/room/resolve.go`,
`internal/hub/{library.go,library_test.go,pawn_test.go}`, `routes.go`,
`templ/pages/{assets.go,assets.templ,asset-tabs.templ,asset-grid.go,notice-panel.go}`,
`templ/pages/pages_test.go`.

**Checkpoint 5.** Added: `internal/room/{tile.go,tile_test.go}`,
`internal/controllers/{room-palette.go,room-palette_test.go}`,
`templ/pages/{room-palette.go,room-palette.templ,room-palette_test.go,room-table-menu_test.go}`,
`js/room/render/terrain-pass.ts`, `js/room/render/stages/{terrain.ts,terrain.test.ts}`.
Changed: `internal/room/{state,derive,reduce,command,snapshot,table,validate}.go` and
their tests plus `scenario_test.go`, `authorize_test.go` and `resolve_test.go`;
`internal/controllers/room-table-menu.go`; `routes.go`;
`templ/pages/{room.go,room-table-menu.go,room-table-menu.templ,rooms_test.go,scenes_test.go}`;
and on the client `model/{overlay,revisions,grid,hex}.ts`, `modes/{switch,table,select,testing}.ts`,
`render/{pawn-pass.ts,shaders/pawn.ts,stages/order.ts}`, `store.ts`, `panels.ts`,
`table-menu.ts`, `main.ts` and their tests.

**Checkpoint 6.** Added: `internal/controllers/room-table-menu_test.go` (which takes
`TestTheRingIsThePaletteInTheOrderTheGMBuiltIt` over from `room-palette_test.go`).
Changed: `internal/room/{state,table,tile,scene}.go`,
`internal/room/{tile,authorize,derive,fixture,scene,scene_load,state}_test.go`,
`internal/controllers/{room-table.go,room-table-menu.go,scenes.go}` and
`internal/controllers/{room-table_test.go,scenes_test.go}`,
`templ/pages/{room-settings.go,room-settings.templ,room-table-menu.go}`,
`templ/pages/{room-table_test.go,room-table-menu_test.go}`, and on the client
`js/room/store.ts` and `js/room/render/layers.test.ts`.

**Checkpoint 7.** Added: `internal/room/{note.go,note_test.go}`,
`internal/controllers/{room-notes.go,room-notes_test.go}`,
`templ/pages/{room-note.go,room-note.templ,room-note_test.go}`,
`js/room/note-window.ts`. Changed: `internal/room/{state,validate,derive,reduce,command,
snapshot,project,scene,table,tile}.go` and their tests plus `scenario_test.go`,
`projection_test.go`, `derive_test.go`, `authorize_test.go` and `scene_test.go`;
`internal/hub/{hub.go,actor.go}`; `internal/controllers/{room-palette.go,
room-table-menu_test.go,scenes_test.go}`; `routes.go`; `templ/pages/{room-table-menu.go,room-table-menu.templ,
room-table-menu_test.go}`; and on the client `store.ts`, `panels.ts`,
`model/revisions.ts`, `table-menu.ts`, `modes/{select.ts,table.ts,testing.ts}`,
`main.ts` and their tests.

**Checkpoint 8.** Added: `js/room/model/{numbering.ts,numbering.test.ts}`,
`js/room/render/stages/{numbers.ts,numbers.test.ts}`, `js/room/note-window.test.ts`. Changed:
`internal/room/state.go`, `internal/controllers/{room-table.go,room-table_test.go}`,
`templ/pages/{room-grid.go,room-grid.templ,room-table_test.go}`, and on the client
`render/path-pass.ts`, `render/stages/{order.ts,order.test.ts,list.test.ts}`,
`note-window.ts`, `main.ts`, plus every grid literal in the test helpers.

Regenerated across all of them: `protocol.ts`, `testdata/reducer/{gm,player}.json`,
`testdata/snapshots/schema-3.json`, `public/css/app.css`, `public/static/room.js`.
