# Hex crawl: progress

Working notes against `plans/hex-crawl.md`. Delete this file when the plan is done.

Branch `yet-another-rewrite`. Checkpoints 1-4 are committed (`edcae11`, `4f17aed`,
`6ddd4a5`); **everything for checkpoint 5 is uncommitted, in the working tree**.
`make check` is green and `make js`, `make css`, `make protocol`, `make sqlc`, `make db`
and `templ generate` have all been run, so `make run` is enough to look at it.

## Where we are

**Checkpoints 1 to 4 are built and verified in a browser.** Checkpoint 5 is built and
unverified. The next session starts by running the CP5 test plan at the bottom of this
file, then moves to Checkpoint 6.

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
5. **The cell brush is a third button in the fog shape group, not a fourth control.**
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
| **CP5** | 6 | The palette, the wheel's ring, the stamp brush and the tile stage. **Built, unverified.** |
| **CP6** | 7 | Players stamp, behind `PlayersCanStamp`. |
| **CP7** | 8 | The hex key: notes per cell, revealed by the GM. |
| **CP8** | 9 | The party pawn reveals fog as it moves. |

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
    painted, under their fog. A player right-clicking empty ground gets **no wheel at
    all** — phase 7 is what opens it to them.
12. **Remove a picture from the bag.** The confirm says the stamped cells go with it.
    They do, on every floor, and the ring loses that button.
13. **Floors.** Stamp on the ground floor, switch floors, stamp there: each floor keeps
    its own tiles. *Tabletop → Clear tiles* empties the floor you are looking at and
    leaves the other alone, and leaves the bag full.
14. **Reload both browsers**, and everything is still there. Save a scene with tiles on
    it, clear the tabletop — the tiles go and **the palette stays** — then reopen the
    scene and both come back.

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

Regenerated across all three: `protocol.ts`, `testdata/reducer/{gm,player}.json`,
`testdata/snapshots/schema-3.json`, `public/css/app.css`, `public/static/room.js`.
