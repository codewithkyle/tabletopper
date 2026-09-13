# Hex crawl: progress

Working notes against `plans/hex-crawl.md`. Delete this file when the plan is done.

Branch `yet-another-rewrite`. Checkpoint 1 is committed as `edcae11`; **everything for
checkpoints 2 and 3 is uncommitted, in the working tree**. `make check` is green and
`make js`, `make css`, `make protocol` and `templ generate` have all been run, so
`make run` is enough to look at it.

## Where we are

**Checkpoints 1, 2 and 3 are built. None of them has been driven in a browser.** The next
session starts by running the three test plans at the bottom of this file, then moves to
Checkpoint 4.

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
| **CP1** | 0, 1, 2 | A hex grid you can see, snap to and measure in. **Built, unverified.** |
| **CP2** | 3 | A layer carries two pictures: the GM's map and the players' map. **Built, unverified.** |
| **CP3** | 4 | The fog brush paints by the cell, square or hex. **Built, unverified.** |
| **CP4** | 5 | Terrain is its own asset kind with its own tab and its own shelf. |
| **CP5** | 6 | The palette, the wheel's ring, the stamp brush and the tile stage. |
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

Regenerated across all three: `protocol.ts`, `testdata/reducer/{gm,player}.json`,
`testdata/snapshots/schema-3.json`, `public/css/app.css`, `public/static/room.js`.
