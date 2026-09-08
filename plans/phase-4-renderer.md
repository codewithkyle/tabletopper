# Phase 4: the renderer foundation

Read `plans/vtt-overview.md` (Client section and the inherited tile
conventions), then `plans/phase-3-transport.md` for the client modules this
builds on. This phase puts a WebGL2 canvas in the room page, draws the current
map from its tile pyramid with pan and zoom, draws the grid, and gives the GM
the dialogs that choose a map and configure the grid. No pawns, no fog, no
strokes yet.

## End state

- The GM opens a Layers dialog: adds, renames, reorders and removes layers,
  chooses a tiled map for each, and sets which layer is active. Every client's
  canvas shows the active layer's map and crossfades when it changes. Removing
  a layer that still has pawns goes through the confirm modal with the pawn
  count in the message and deletes them.
- The GM can view a layer other than the active one to prepare it, with a
  clear indicator and a button to make it active. Players always see the
  active layer and its name.
- The GM opens a Grid dialog, changes cell size, offsets, colour, snapping,
  feet per cell and diagonal rule, and every client's grid updates.
- Everyone can pan with a drag or middle button, zoom to the cursor with the
  wheel, pinch on a touchscreen, and reset the view. Tiles load progressively
  with a coarser parent shown until the child arrives. There is no visible
  seam between tiles at any zoom.
- A 12000 by 9000 map at any zoom renders in under two milliseconds of CPU
  time per frame on a laptop, and the frame loop is idle when nothing changes.
- The debug panel gains a benchmark button that sweeps the camera for ten
  seconds and reports average and 95th percentile frame time.

## Decisions

1. **Level 0 is native and `z` increases as detail falls.** This is the inverse
   of the slippy-map convention and it is what the pyramid on the server uses.
   Mixing conventions renders a level off and looks like a blurry map rather
   than an error.

   **CORRECTED WHILE BUILDING: a tile's native rectangle is NOT
   `x * tileSize << z`.** `tiler.Build` resizes each level to
   `LevelPixels(size, z)` -- a CEIL -- and that level then stands for the whole
   map, so the scale is `size / levelPixels(size, z)` per axis per level and
   only equals `2^z` when the dimension divides evenly. 9000 rows at level 5 is
   282 pixels, and 282 x 32 is 9024: the shift draws the map 24 pixels taller
   than it is, so the grid slides off the features it lines up with and the map
   breathes as the level changes. `tiles.levelScale` is the fix and
   `tiles.test.ts` pins it.
2. **Pyramid math is a line-for-line port of `internal/tiler/pyramid.go`**,
   with the same test example: 12000 by 9000 at tile size 512 gives
   `maxZoom` 5, six levels, 584 tiles. `ceil`, never a shift; edge tiles are
   the remainder, never padded.
3. **Level selection is `round(log2(1 / (zoom * devicePixelRatio)))`**, clamped
   to `[0, maxZoom]`, in a single constant-bearing function so it can be tuned.
   Device pixel ratio is part of the scale because a retina display at zoom 1
   is showing native pixels at half size.
4. **The tile cache is one `TEXTURE_2D_ARRAY`** of `tileSize` square RGBA8
   layers, allocated with `texStorage3D`, 96 layers by default and never more
   than `MAX_ARRAY_TEXTURE_LAYERS`. LRU by last frame used. An edge tile
   occupies part of a layer and its quad samples up to `tileW / tileSize`.
   Filtering is `LINEAR` with `CLAMP_TO_EDGE`, no mipmaps; level selection is
   the mip.
5. **Tiles fetch by priority and abort on level change.** A queue ordered by
   distance from the viewport centre, at most eight requests in flight,
   `AbortController` per request, cancelled when the tile is no longer wanted.
   The fetch path is `fetch` with same-origin credentials, `blob`,
   `createImageBitmap` with `premultiplyAlpha: "none"` and
   `colorSpaceConversion: "none"`, `texSubImage3D`. A 404 marks the tile
   missing for the generation and is never retried.
6. **The grid is a shader, not geometry.** A full-viewport quad whose fragment
   shader maps its pixel back to map space through the inverse camera and
   draws a line where the fractional cell coordinate is within one device
   pixel of an edge, using `fwidth` so lines stay one pixel wide at every zoom.
   Colour and alpha from the grid colour.

   **CORRECTED WHILE BUILDING: the grid is NOT clipped to the map rectangle.**
   This plan said it was, and the old client's shader never did. Play leaves
   the image constantly -- a chase off the north road, a camp in the woods, a
   party backed against the edge of the drawn area -- and whether somebody is
   five feet or thirty feet off the picture is a question the grid is the only
   thing that answers. Stopping the cells at the image turns the surrounding
   table into a place where a pawn cannot be positioned, only dropped. So the
   grid is infinite: it covers the viewport at every camera position, the map
   rectangle is not passed to the pass at all, and the fade at two to six
   device pixels per cell is the only thing that ever ends it.
7. **The frame loop is driven by a dirty flag.** `invalidate()` requests a
   frame if none is pending. A frame renders, then requests another only if
   something is still pending: a drag in progress, a tile upload queued, an
   animation running. An idle room costs zero frames.
8. **Every DOM control goes through HTTP.** The Layers and Grid dialogs are
   fragments in the content modal that post to room routes; the handler builds
   the command and `hub.Dispatch`es it. The canvas learns of the change from
   the `table.updated` event like every other client. No command is sent from
   the dialog over the socket.
9. **Camera state is world-centred.** `{ x, y, zoom }` where `(x, y)` is the map
   pixel at the viewport centre. `screen = (world - camera) * zoom + viewport / 2`
   in CSS pixels, multiplied by the device pixel ratio for the canvas. One
   function each way, tested by round trip.
15. **The outgoing map is drawn OPAQUE and the incoming one dissolves over it**,
    not "the old at falling alpha under the new at rising alpha" as decision 11
    below first put it. Complementary alphas are wrong over a cleared buffer:
    old at `1-t` then new at `t` leaves
    `t*new + (1-t)^2*old + t(1-t)*background`, which is a quarter of the table
    colour showing through at the midpoint -- the transition dips through the
    empty desk and back. Painting the old one solid and dissolving the new one
    over it is exactly `lerp(old, new, t)`.
16. **Switching to a floor of the same size does not move the camera.** The plan
    said fit runs "when the map changes". Floors of a building are aligned --
    that is why the grid is room-wide -- so a GM stepping from the ground floor
    to the cellar is looking at the same corner of the same building, and
    re-fitting would throw away the one thing they were doing. Fit runs for the
    first map of the session and for a map of a different SHAPE.

11. **One layer is rendered at a time, and the client tracks a viewed layer.**
    For players it is always the active layer from the store. For the GM it is
    a local choice, defaulting to the active layer and following it whenever
    the active layer changes. Every pass reads the viewed layer: its map for
    tiles, its rectangle for the grid clip and camera fit, and in later phases
    its pawns, fog and strokes. Tiles are cached by map id, so switching keeps
    the old layer resident for a 250 ms crossfade: the old map's tiles draw at
    falling alpha under the new map's at rising alpha.
12. **The layer manager and the grid settings are WINDOWS, not modals.** The
    overview's Windows section names both of them as examples, and it was
    written after this plan's first draft; where the two disagree the overview
    wins. A grid tuned by eye must not cover the table it is being tuned
    against, and a GM preparing a floor parks the manager open while they work.
    The map picker stays a content modal: it is a one-shot pick that closes on
    choose, which is what a modal is for.
13. **The viewed layer is shown in the menu bar**, at its right end: for the GM
    a `<select>` of the layers with a "Viewing, not active" badge and a Make
    active button when the viewed and active layers differ; for a player a plain
    label naming the active layer. It is the one piece of table state that has
    to be glanceable -- a GM who forgets they are on the cellar moves pawns onto
    a floor nobody can see -- so it is not behind a menu.
14. **Every layer mutation answers 204 and the manager refetches on the event.**
    `table.updated` reaches every client already, so `panels.ts` raises
    `room:table` and the manager carries
    `hx-trigger="room:table from:window"`. That is the refetch pattern the
    player list uses, and it is what keeps a GM's second tab correct. Replying
    with the re-rendered list instead would save a round trip and leave the
    other tab stale.

17. **Snapping is a whole-cell step or a half-cell one, and the third choice is
    off.** The dialog first offered Cells, Corners and Off, where Corners was
    the parity rule inverted -- odd footprints on vertices, even ones on
    centres. Nobody wants that: a table that plays on intersections still wants
    a large creature to cover four whole squares, so inverting the parity moved
    the wrong pawns. What the third option was reaching for is a lattice that
    holds BOTH, and stepping by half a cell is exactly that, because the odd
    multiples of half a cell are the centres and the even ones are the
    vertices. So the modes are `cells` (Centre only -- the 5e parity rule, a
    creature in the middle of the squares it fills), `halfCells` (Centre and
    corners -- the nearest of either, footprint irrelevant) and `off`. The
    parity term in `SnapAxis` survives for `cells` alone and `halfCells` needs
    no phase at all. `cells` keeps its wire value, so no stored room moves.

10. **Zoom range is 0.05 to 4.** The old client stopped at 0.1 and 2 because it
    had one texture. Tiles make a full-map overview cheap. Wheel zoom applies
    a multiplier capped at 25 percent per event, anchored at the cursor.

## Server

### Routes

| Pattern | Wrapper | Handler | Behaviour |
| --- | --- | --- | --- |
| `GET /fragment/room/layers?room={id}` | `Fragment` | `RoomLayersFragment` | GM only. The layer manager, opened as a window from the Tabletop menu. One row per layer, bottom to top, with a name field (`hx-patch` on change), the map's preview and name or "No map", Choose map, Clear map, Up, Down, an Active radio, and Remove. Remove carries `hx-confirm` reading "Delete <name> and the N pawns on it? Their fog and drawings go with them. This cannot be undone.", `data-confirm-label="Delete layer"`; when the layer has no pawns the message drops the pawn clause. An Add layer form with a name field sits below the list. A warning line appears under any layer whose map dimensions differ from the first layer's. It carries `hx-trigger="room:table from:window"` and refetches itself; every mutation below answers 204. |
| `POST /rooms/{id}/layers` | `RequireSession` | `AddLayer` | GM only. Dispatches `table.addLayer`. |
| `DELETE /rooms/{id}/layers/{layer}` | `RequireSession` | `RemoveLayer` | GM only. Dispatches `table.removeLayer`. The core deletes the pawns; the confirm text already said so. |
| `PATCH /rooms/{id}/layers/{layer}/name` | `RequireSession` | `RenameLayer` | GM only. Dispatches `table.renameLayer`. |
| `POST /rooms/{id}/layers/{layer}/move` | `RequireSession` | `MoveLayer` | GM only. Reads `index`, dispatches `table.moveLayer`. |
| `POST /rooms/{id}/layers/{layer}/activate` | `RequireSession` | `ActivateLayer` | GM only. Dispatches `table.setActiveLayer`. |
| `GET /fragment/room/maps?room={id}&layer={id}` | `Fragment` | `RoomMapsFragment` | GM only. Pick-shaped cards of the owner's maps with a generation: preview image, name, dimensions. Each card is a `<form>` posting to the layer's map route with the asset id in a hidden field. Close first. |
| `POST /rooms/{id}/layers/{layer}/map` | `RequireSession` | `SetLayerMap` | GM only. Reads `asset`, resolves via `GetMapPyramid` (owner must be the GM, `tile_gen` must be set, else `htmx.Error` "That map is not ready yet" 409), `hub.Dispatch(table.setLayerMap)`, `htmx.CloseModal` -- the picker is the one content modal here. |
| `DELETE /rooms/{id}/layers/{layer}/map` | `RequireSession` | `ClearLayerMap` | GM only. `hub.Dispatch(table.clearLayerMap)`. |
| `GET /fragment/room/grid?room={id}` | `Fragment` | `RoomGridFragment` | GM only. Opened as a window from the Tabletop menu. A form pre-filled from the live state via a new `hub.Table(roomID)` accessor, which returns the table and a pawn count per layer -- the manager needs the counts for its confirm text and this needs the grid. |
| `POST /rooms/{id}/grid` | `RequireSession` | `SetRoomGrid` | GM only. Parses the eight grid fields plus `monsterHp` and `playersCanDraw`, dispatches `table.setGrid` then `table.setOptions`, and answers 204. A window has no `htmx.CloseModal` and wants none: the GM adjusts, watches the table, adjusts again. Validation errors come back from the core as `room.Error` and are rendered with `PanelFormErrors` at 422, matching the character panels. |

The resolution for `table.setLayerMap` lives in `hub/resolve.go`, replacing the
stub from phase 3, so the same code path serves a future socket-originated
set-map if one is ever wanted.

New query in `server/sql/assets.sql`:

| Name | Kind | Statement |
| --- | --- | --- |
| `ListReadyMaps` | `:many` | `SELECT id, name, width, height FROM assets WHERE owner_id = ? AND type = 'map' AND tile_gen IS NOT NULL ORDER BY name` |

### Templates

- `pages/room-layers.templ`, `.go`: `RoomLayersFragment(data RoomLayersData)`
  and the `layerRow` partial. `RoomLayersData` carries, per layer, the name,
  the map's preview URL and name, its dimensions, whether it is active, the
  pawn count (from the live state, for the confirm text), and whether its size
  differs from the first layer's. The controller composes the confirm string;
  the template only prints it.
- `pages/room-maps.templ`, `.go`: `RoomMapsFragment(data RoomMapsData)`; a card
  is a `<form hx-post=... hx-swap="none">` with an `<img>` preview at
  `/assets/images/{id}/preview` and a `<button>` covering the card. Buttons are
  solid `btn`; Close first.
- `pages/room-grid.templ`, `.go`: `RoomGridFragment(data RoomGridData)`; a form
  with number inputs for cell size, offsets and feet per cell, a colour input
  (an 8-digit hex text field with a native colour picker beside it is fine;
  the core accepts 6 or 8 digits), radio groups for snap and diagonals, a
  toggle for visible, a select for monster HP visibility, a toggle for players
  can draw. The `hx-post`, `hx-target`, `hx-swap`, `hx-status:422` trio
  targets its errors block; pin it in a template test.
- The MENU BAR, not a header of buttons: this plan predates the bar and the
  windows, and both are built. The **Tabletop** menu's four disabled
  placeholders become `Layers` and `Grid & settings`, each a `RoomMenuItem`
  carrying a `RoomWindow`, beside `Spawn pawns` and `Clear tabletop` which stay
  disabled. The **View** menu's five disabled items become `Action` items --
  `Zoom in`, `Zoom out`, `100%`, `200%`, `Fit map` -- beside the fullscreen
  toggle that already works.
- The bar's right end gains the layer indicator: for players a label with the
  active layer's name, filled by the client from the store; for the GM a
  `<select>` whose options the client rebuilds from the store, choosing the
  viewed layer, with a "Viewing, not active" badge and a Make active button
  (posting to the activate route) shown whenever the two differ. The select and
  badge are rendered in templ; the script fills options and toggles `hidden`.
- The View items and the camera are IN DIFFERENT BUNDLES and cannot import each
  other: the bar is `server/public/js/room.js`, the renderer is
  `server/js/room/render/`. `room.js` gains one case per action that dispatches
  `room:view` on window with `{ action }`, and the renderer listens. The event
  name is a cross-bundle contract and is spelled out in both files, the way
  `"alert:pending"` is.

The page-data type for the room adds nothing for maps: the client learns every
layer's pyramid from the snapshot's `table.layers`.

## Client

### Modules under `server/js/room/render/`

```
gl.ts        context creation, program compile and link helper, uniform lookup, error checks in development only
camera.ts    Camera state, worldToScreen, screenToWorld, zoomAt, fit, clamp
pyramid.ts   levelPixels, levelTiles, levelTileSize, maxZoom, tileRect: the Go port
tiles.ts     TileCache (texture array, LRU), TileLoader (queue, abort), visible-range and ancestor lookup
tile-pass.ts the instanced tile draw
grid-pass.ts the grid quad
frame.ts     the dirty-flag loop, resize handling, frame timing for the benchmark
input.ts     pointer, wheel and touch to camera changes and to a small event API for later phases
layers.ts    the viewed layer: follows the active layer, GM override, crossfade timing
renderer.ts  owns the passes, takes the store, subscribes to table changes
```

### Setup

`renderer.ts` is constructed with the mount element and the store. It creates
a `<canvas>` inside the mount, gets a `webgl2` context with
`{ alpha: false, antialias: false, depth: false, stencil: false, preserveDrawingBuffer: false, powerPreference: "high-performance" }`,
and installs a `ResizeObserver` that sizes the canvas to the mount times the
device pixel ratio and invalidates. It reads the theme's base colour from the
computed style of the mount (`background-color`) for the clear colour and
re-reads it on the `theme:change` window event. A missing WebGL2 context
renders a message into the mount from a templ-rendered hidden element and
stops; the rest of the room page keeps working.

### Camera and input

- Pan: primary button drag when no tool is active (there are no tools yet, so
  always), middle button drag always, one touch. Pointer events with
  `setPointerCapture`; handlers store the pointer position and set a flag; the
  frame applies it.
- Wheel: `zoomAt(screenX, screenY, multiplier)` with the multiplier clamped to
  `[0.75, 1.25]` from `deltaY`, normalised for `deltaMode` lines and pages.
- Pinch: two active pointers; scale by the ratio of their distance, anchor at
  their midpoint, pan by the midpoint's movement.
- Reset view and Fit map: a window event `room:view` with `{ action }` that the
  header buttons dispatch with `hx-on:click`; the renderer listens. Fit sets
  zoom so the map fills the viewport with a margin and centres it. Fit runs
  automatically the first time a map appears in the state and when the map
  changes.
- Zoom is clamped to `[0.05, 4]`; the camera position is clamped so the map
  cannot be panned more than half a viewport off screen when a map is set.

### Tile pass

Per frame, for the viewed layer's map when it has one, and during a crossfade
for the previous layer's map as well at its fading alpha:

1. Compute the level from the camera and the device pixel ratio.
2. Compute the visible map rectangle from the camera, intersect with the map,
   convert to a tile index range at that level with `pyramid.ts`.
3. For each tile in range: if resident, add an instance for it; otherwise walk
   ancestors `(z+1, x>>1, y>>1)` until one is resident and add an instance for
   the sub-rectangle of the ancestor that covers this tile, then request the
   tile from the loader with its priority.
4. Upload one instance buffer: for each instance, the map-space rectangle, the
   layer index, and the UV rectangle. Instances at different levels are drawn
   in the same call because the array texture holds every level; the UV
   rectangle handles the partial coverage.
5. One `drawArraysInstanced` of a unit quad. The vertex shader takes the camera
   as a `mat3` uniform and the map-space rectangle as instance attributes.
   The fragment shader samples `sampler2DArray` at the interpolated UV and
   layer.

The loader uploads at most four tiles per frame so a burst of arrivals never
stalls a frame; the rest wait for the next frame and keep the loop alive.

THE CACHE IS KEYED BY TILE SIZE AS WELL, and holds one texture array per
distinct size. `assets.tile_size` is a per-row column, stored deliberately so a
map tiled under an old constant keeps working; a texture array has one fixed
layer size, so two maps tiled differently cannot share one. In practice there
is exactly one array.

Tile URLs are built from the layer's `map` and the pyramid math:
`/assets/maps/{assetId}/tiles/{gen}/{z}/{x}_{y}.webp`. The cache is keyed by
asset id and generation as well as tile coordinates, so several layers' maps
share it and the LRU decides what stays. When a `table.updated` changes a
layer's `gen` or `assetId`, that map's entries are dropped and any queued
fetches for it are abandoned. When the viewed layer changes, the loader's
priority moves to the new map and the old one is left to age out.

### Grid pass

A second program drawing one full-viewport triangle after the tiles when
`grid.visible`. Uniforms: inverse camera, cell size, offsets, colour, device
pixel ratio -- and no map rectangle, per decision 6. The fragment shader
computes map coordinates, computes distance to the nearest vertical and
horizontal cell edge in device pixels via `fwidth`, and blends the colour with
a one-pixel line. It covers the whole viewport whether or not the viewed layer
has a map, so a GM sizing cells before choosing a map sees the same grid a
party standing off the north edge of one does.

### Frame timing

`frame.ts` records the CPU time of each frame with `performance.now()` around
the render into a ring of 600 samples. The debug panel's benchmark button runs
a scripted camera sweep (pan across the map at three zoom levels, then zoom in
and out at the centre) for ten seconds and prints average and 95th percentile
CPU frame time and the number of tiles fetched. The result is the first
measurement the overview asked for; keep the numbers in the pull request.

## Tests

**TypeScript** (`node --test`):

- `pyramid.test.ts`: the Go example (12000 by 9000, 512: `maxZoom` 5, tile
  counts 24 by 18, 12 by 9, 6 by 5, 3 by 3, 2 by 2, 1 by 1, total 584) and
  `levelPixels(9000, 5) === 282`.
- `camera.test.ts`: `screenToWorld(worldToScreen(p)) === p` at several zooms
  and ratios; `zoomAt` keeps the anchor fixed; clamps hold.
- `layers.test.ts`: the viewed layer follows the active layer until the GM
  overrides it and snaps back when the active layer changes; the crossfade
  reports both maps with complementary alphas for 250 ms and then only the new
  one; a player cannot override.
- `tiles.test.ts`: visible range at the map's edges; ancestor lookup returns
  the nearest resident ancestor and its UV sub-rectangle; LRU evicts the least
  recently used layer and never a layer used this frame; level selection table
  at zooms 1, 0.5, 0.3, 0.1 with ratios 1 and 2.

**Go**:

- `RoomMapsFragment` lists only maps with a generation and only the owner's.
- `SetLayerMap` refuses another user's map and an untiled map with the right
  messages, and dispatches the resolved command with the pyramid values from
  the row and the layer id from the path.
- `RoomLayersFragment` renders the confirm text with the pawn count, drops the
  pawn clause at zero, and shows the size warning only for a differing layer.
- `RemoveLayer`, `MoveLayer`, `ActivateLayer` dispatch the right commands and
  refuse a player.
- `SetRoomGrid` turns a core `invalid` into a 422 with the field's message and
  a valid form into two dispatches in order.
- Template test pins the grid form's trio.

## Build order

Three checkpoints, agreed with the user, each stopping for review:

1. **The canvas**: `gl.ts`, `camera.ts`, `frame.ts`, `input.ts`, `grid-pass.ts`
   and a `renderer.ts` that draws only the grid. A pannable, zoomable grid over
   an empty table, with the View menu wired. NO SERVER CHANGE AT ALL, which is
   what makes it the first one: the riskiest part of the phase is brought up
   against nothing.
2. **The controls**: `resolve.go`, the seven routes, the three fragments, the
   two windows and the layer indicator. A map can be put on a layer and the
   grid configured, and the canvas still only draws the grid.
3. **The tiles**: `pyramid.ts`, `tiles.ts`, `tile-pass.ts`, `layers.ts`, the
   crossfade and the benchmark.

`make js-test` runs `node --test ./server/js/room/*.test.ts`, a glob that does
not descend into `render/`. It is fixed in checkpoint 1 or every test below is
silently not run.

## Verification

The GL path has no unit tests -- a shader that fails to compile or a UV that is
transposed is a black screen, not an assertion -- so it is checked by driving
the real renderer in headless Chromium against synthetic tiles whose colour
names their own (z, x, y). A pixel read back off the canvas then says exactly
which tile of which level landed there. Three things make that harness work:

- **requestAnimationFrame is replaced with a queue the probe pumps.** Headless
  services only TWO rAF callbacks under `--virtual-time-budget`, and the
  loader's path is three frames long (request, upload, draw).
- **Every read happens in the same task as the draw.** `preserveDrawingBuffer`
  is false, so the moment the probe yields, the compositor may take the buffer
  and a read returns a cleared one.
- **`OffscreenCanvas.convertToBlob` never resolves under virtual time**, so the
  tile's colour rides in the blob's MIME type and a stubbed `createImageBitmap`
  builds the ImageData from it. Both stubs are synchronous.

Bundle the probe with `esbuild --format=iife`, inline it into one HTML file,
and open it as `file://` (a local HTTP server cannot bind a port here); report
through `document.title` and read it with `--dump-dom`.

1. `make js && make check`, then the CSS selector diff for the new templates.
2. Open a room with a large tiled map. Zoom from the whole map to native
   detail; tiles resolve coarse to fine, no seams, no gaps at the right and
   bottom edges. Pan fast; no more than eight requests in flight in the network
   panel, cancelled requests visible as the level changes.
3. Turn on the grid; change cell size and offset; lines are one pixel wide at
   every zoom and align with the map's own grid when the offset is right.
4. Run the benchmark on the largest map available and record the numbers.
5. Chrome's performance panel over a minute of idling shows no frames.
6. A second client sees the map and grid change with no reload.
7. Add a "First floor" layer, give it a map, make it active: both clients
   crossfade to it. The GM switches their view back to the ground floor; the
   player's canvas does not change and the GM sees the viewing badge. Remove
   the first floor while it is active: the confirm modal names it, and both
   clients land on the ground floor.

## Out of scope

Pawns, fog, strokes, pings, any hit testing, the spawn placement mode, and any
DOM overlay positioned in map space. `input.ts` exposes pointer events in map
coordinates for phase 5 to consume but nothing consumes them here.
