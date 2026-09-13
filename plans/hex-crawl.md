# Hex grids, tiles and hex crawls

Written 2026-09-12 from a read of `server/internal/room/{state,snap,validate,fog,stroke,pawn,project,derive}.go`,
`server/internal/controllers/{map-tiles,library-assets,room-fog}.go`,
`server/js/room/{tools,fog-tool}.ts`, `server/js/room/model/{grid,shape,polygon}.ts`,
`server/js/room/modes/{place,measure,fog}.ts`,
`server/js/room/render/{sprites,glyphs,shaped-quad,pawn-pass}.ts`,
`server/js/room/render/shaders/{grid,pawn}.ts` and `server/js/room/render/stages/{order,list,pawns}.ts`.

Audited 2026-09-12 against `server/internal/room/{state,derive,fog,stroke,pawn,table,resolve,validate,snap,project,snapshot}.go`,
`server/internal/hub/{actor,effects}.go`, `server/internal/controllers/map-tiles.go`, `server/routes.go`,
`server/js/room/{tools,dialogs,fog-tool,draw-tool,window,panels}.ts`,
`server/js/room/modes/{switch,place,select,table}.ts`, `server/js/room/model/{grid,shape,overlay,revisions}.ts`,
`server/js/room/render/{layers,pawn-pass,sprites,input,glyphs}.ts`, every stage and shader,
`server/templ/pages/{room,room-grid,room-spawn}.templ`, `server/templ/pages/room.go` and the vendored
`server/css/vendor/daisyui.js` (5.7.28). The audit took stamping off the tool bar and onto a radial
menu under the pointer, which is also the table context menu the scenes plan builds, took the stamp
out of the `Mode` union, moved the palette off `TableSettings`, moved the
auto-reveal out of `hub/effects.go` and into `PawnMove.Apply`, dropped the claim that picture asset
ids are secret, and corrected a handful of paths. Kyle's call the same day: tile art is its own
asset kind with its own section, never a token.

Everything here is prep, and prep without somewhere to keep it is a session
that has to be rebuilt every week, so `plans/scenes.md` runs first. The
dependency is narrower than "all of it", and an agent starting cold can begin
here: phases 0, 1, 3, 4 and 5 below wait for nothing in the scenes plan; phase
2 wants scenes phase 1, because it edits the *Grid* window that phase splits
out (against the unsplit *Grid & settings* form it is the same edit in a
bigger file); phases 6 to 9 want scenes phase 5, which builds the wheel their
menu items hang on, and scenes phase 0, whose export and import lists they
extend. The four places the two plans touch are listed at the end under *Where
this meets the scenes plan*.

A hex crawl is a party crossing a wilderness hex by hex, learning what is in each
one. Four things make that work, and none of them is a rule:

1. the map is drawn in hexes, and distances are read in hexes or miles;
2. the players' copy of the map is blank and fills in as they explore;
3. each hex has something written behind it that the GM reveals when they get there;
4. players mark the map themselves — stamping terrain, drawing routes, circling
   where they mean to go.

**No travel rules are encoded here.** Watches, navigation checks, forage, weather
and encounter dice disagree between every ruleset that has them, and anything
built in would be wrong for most tables. The GM rolls those in the dice tray like
everything else.

## What is already true

**The grid is one fullscreen fragment shader.** `shaders/grid.ts` maps a clip
quad to world space and does the whole grid in `fract()` with `fwidth()`
antialiasing and a zoom fade, from four uniforms: `u_offset`, `u_cell`,
`u_color`, `u_dash`. A hex grid is the same pass with different maths; no
geometry, no buffers, no new stage.

**The pawn shader already branches on a mask.** `shaders/pawn.ts` has three: a
circle with a border, wounding and a heartbeat for creatures; a plain sprite for
objects; a ringed blood stain. `pawn-pass.ts:20-22` names them `SHAPE_DISC`,
`SHAPE_RECT` and `SHAPE_STAIN`. A hexagon is `SHAPE_HEX = 3` and a fourth
`else if`, which is why tile art does not need to be masked at upload and can
stay orientation-agnostic.

**The sprite cache is keyed by URL.** `sprites.ts` holds a 128-layer, 256px
texture array keyed by URL, evicted least-recently-used by
`gl/texture-array.ts`, so a thousand stamped tiles drawn from twenty pictures
cost twenty slots.

**Fog is polygons in world space and needs no model change.** `FogShape` is a
`rect` or a `poly` with a flat `Points` list; a revealed hex is a six-point
`poly` reveal. `FogPointsBudget` is 200,000 and `FogShapesMax` is 2,000, so a
30×30 hex map fully revealed is 5,400 points and 900 shapes — comfortable, and
worth knowing before designing something cleverer. Fog has no undo: shapes are
added (`FogAdd`), removed (`FogRemove`) and cleared (`FogClear`), and that is
the whole vocabulary.

**Drawing already works for players.** `PlayersCanDraw` is a table setting,
`StrokeBegin` checks it at `stroke.go:52`, and the tool does free, rect, circle
and cone. Routes and circled areas are a feature you have today; they just do
not snap to hexes.

**Place is not a mode, and it is the model for stamping.** `tools.ts` is a
six-way `Mode` union mounted from `[data-room-tool-*]` buttons, and `place` is
not in it. `modes/switch.ts:38` routes every press to `place` while it is armed,
`dialogs.ts` arms it from the spawn dialog's `data-spawn-pick` cards and closes
the modal, `place.ts` sends a spawn on every press and stays armed, and a
secondary click or Escape puts it away. The stamp brush is armed the same way,
from the wheel. The tool bar does not change.

**Map tiles are not access-controlled.** `GET /assets/maps/{id}/tiles/...` is
behind `auth.RequireSessionOr404`, and `GetMapTile` then checks only that the
tile exists. It does not check ownership or room membership — so anyone signed
in who learns an `(assetId, gen)` pair can walk that whole pyramid by guessing
`z/x/y`. That is not a problem today because a player is only ever told about
maps they are meant to see. **It becomes the central constraint of phase 3.**

**Picture URLs already name their asset.** A token's image is
`/assets/images/{id}`, and every pawn carries one to every player today. A
picture's asset id is not a secret and this plan does not pretend it is; a map's
pyramid is the thing to guard.

**`Table` is `Layers` plus an embedded `TableSettings`, and `Derive` treats the
two differently.** `TableSettings` is compared with `!=`, so it must stay a
comparable struct — no slices in it, ever. `Layers` is compared with
`reflect.DeepEqual` and reported as `LayersUpdated`. Anything list-shaped that
belongs to the table goes beside `Layers` with its own change, and the palette
does exactly that.

**`hub/effects.go` runs inside the actor, after `exec`.** It writes vitals
through to sheets and drops kicked players; it has no way to dispatch a
command back into the room. Anything that must change state as a consequence
of a move happens inside that move's `Apply`, where `Derive` picks it up for
free.

**The pawn context menu is the model for anything that opens under the
pointer.** `pawn-menu.ts` is static markup in `room.templ`, shown at the screen
point `select.ts` hands it on a secondary click over a pawn, flipped to the
other side of the pointer when it would leave the pane, and closed by an
outside press, a scroll or Escape. Over empty
ground `select.ts` returns `false`, and `switch.ts` lets any tool's `secondary`
fall through to it. That `false` is where the table's own menu hangs.

**daisyUI's flower is four buttons, but its trick is one line of CSS.**
`fab-flower` places each child with
`translate(calc(cos(var(--degree)) * var(--position)), calc(sin(var(--degree)) * -1 * var(--position)))`
from two custom properties, then hides every child past the sixth. The
component cannot hold twenty tiles; the placement can hold any number, and it
is what the wheel uses.

**The asset manager already has a library kind.** `library-assets.go`'s
`libraryKind` is a type, a slug, a storage key, a fit function and two
templates; tokens and avatars are two values of it and share every handler,
and `sql/assets.sql` takes the type as a parameter on every library query. A
third kind is a value, five routes and a tab.

**Tailwind reads `.templ` only.** `@source "../templ/**/*.templ"` is the whole
list. A class named in `server/js` or `server/public/js` is never emitted.

## The shape

1. **Hex is a property of the grid, not a second kind of table.** One `Grid`
   type with a `type` field. Every tool, every snap and every measurement asks
   the grid what shape it is; nothing anywhere asks "are we in hex crawl mode".
2. **Tiles are terrain, not pawns.** They are stamped into cells, one per cell,
   replaced rather than moved, and they have their own cap. A pawn is a thing
   with hit points that somebody drags; a tile is part of the floor.
3. **Everybody stamps from the scene's palette.** A player cannot browse the
   GM's library. The palette is the bag of tiles from the table, made explicit,
   and the GM stamps from the same bag the players do.
4. **The GM's map and the players' map are two images, and the players' client
   is never told the GM's.** Not dimmed, not fogged, not fetched-and-hidden. Not
   sent.
5. **A hex's contents are text until the GM reveals them.** The key is prose in
   the scene, and an unrevealed note's body does not leave the server.

## The hazards worth naming up front

**The tile pyramid leak.** Any design where players see "through" to the GM's map
— fog that reveals the GM layer, a hex window onto the detailed version — hands
the player client an asset id and a generation, and `GetMapTile` will then serve
it every other tile in the pyramid. Revealing terrain to players means **stamping
it onto their map**, which is leak-proof and is what the table did in Tabletop
Simulator anyway. Phase 3 carries a projection test that fails if a GM map ref
ever appears in a player projection.

**Rotation and centring change meaning on hexes.** `SnapAxis` centres a pawn
whose footprint is odd and puts an even one on an intersection. Hexes have no
intersections worth sitting on: everything centres in a cell and size only scales
the sprite. `Diagonals` becomes meaningless and must disappear from the grid
window rather than sit there doing nothing.

**The wheel has to fit under the pointer.** The palette is a ring of picture
buttons around the cell you right-clicked. At 40px buttons on a 48px pitch the
ring's radius is the count times 48 over 2π: twenty tiles sit on a 153px
radius, twenty-four on 183px, and the whole wheel is then about 415px across.
That still fits a 768px-tall pane, and it is the cap: `PaletteMax` is 24 for
this reason and no other. Near an edge the wheel shifts inward rather than
flipping, because a ring cut in half is unusable, and the cell it was opened on
stays outlined so the shift never hides what you are choosing for. A second
ring could double the cap later; nothing here forbids it.

**The sprite array is shared and small.** 128 slots at 256px, shared between tile
art, monster portraits, character portraits and glyphs. A full palette of 24 is
nowhere near it; the symptom of getting near it is eviction churn, not an error,
and the Renderer debug window already reports evictions.

**Note prose is state.** Notes ride in the room snapshot, are diffed by `Derive`
and are re-sent on every reconnect. Without a byte budget a campaign's key would
quietly double the snapshot. Phase 8 sets one, in the idiom `fogBudget` and
`strokeBudget` already use.

## How each phase is run

- Tests are written or changed first. `make check` is red at the end of that step
  for the reasons the phase names and for no other reason.
- Then the code, until `make check` is green.
- `make check` is `fmt-check vet test js-test`; the room bundle is `make js`; the
  wire types are `make protocol` (`go generate ./...` from `./server`), and
  `internal/room/gen/main_test.go:TestProtocolTypesAreCurrent` fails if they are
  stale.
- A change to `internal/room/reduce.go` fails `js-test`: regenerate the fixtures
  with `go test ./internal/room -update` and follow them in `store.ts`.
- A new slice on `State` or `Table` joins `Clone` (`snapshot.go:143`,
  `command.go:CloneTable`), `Normalize`'s `emptied` calls, and
  `revisions.ts`; `TestDerivedEventsConvergeOnTheServersState` is what proves
  the client reducer keeps up.
- No comments, as ever.

---

## Phase 0: the grid grows a type and a unit

### Tests first

- `server/internal/room/validate_test.go`: `checkGrid` rejects an unknown type
  and an unknown unit; accepts all three types and all four units, alongside
  the six checks it already makes.
- `state_test.go`: `Normalize` fills an empty type with `square` and an empty
  unit with `feet`, so **every existing snapshot and every saved scene keeps
  working with no migration and `room.Schema` stays at 3**. Assert that against
  `testdata/snapshots/schema-3.json`, which predates the field.
- `internal/room/gen/main_test.go` covers the wire types by construction;
  `make protocol` regenerates `protocol.ts`.

### Then

- `Grid` gains:

  ```go
  Type  GridType  `json:"type"`
  Units GridUnits `json:"units"`
  ```

  `GridType` is `square`, `hexPointy`, `hexFlat`. `GridUnits` is `feet`,
  `miles`, `kilometres`, `cells`. Both follow the `Values()`/`Valid()` idiom every
  other enum in `state.go` uses, so the generator picks them up unaided, and
  `Normalize` fills them the way it fills `Snap` and `Lines`.
- `FeetPerCell` keeps its name and column meaning "distance per cell" — renaming
  it costs a migration of every snapshot for no behaviour. The grid window's
  label changes; the field does not.
- `CellSize` on a hex grid is the **width across flats** — the dimension that
  makes a 64px hex sit in the same space as a 64px square, so switching a scene's
  grid type does not resize everything on it.

## Phase 1: hex arithmetic

Pure functions on both sides. No rendering, no UI, no wire.

### Tests first

- `server/internal/room/snap_test.go`: a point inside a known hex snaps to that
  hex's centre for both orientations; a point on a shared edge resolves
  deterministically; `SnapOff` returns the input unchanged; footprint parity has
  no effect on a hex grid.
- `server/js/room/model/grid.test.ts`: the same cases, same numbers. The two
  implementations must agree, because the client previews the move and the server
  decides it — they disagree by a pixel and pawns visibly jump on release.
- Distance: cube distance for hexes, unchanged `cellsMoved` for squares;
  `hexLine` returns the cells a straight path crosses, capped like `supercover`.

### Then

- `server/internal/room/hex.go` and `server/js/room/model/hex.ts`: axial and cube
  coordinates, `hexAt`, `hexCentre`, `hexRound`, `hexDistance`, `hexLine`,
  `hexCorners`. Standard cube-coordinate treatment; the only decisions are
  orientation and that `hexCentre` is in the same world pixels everything else
  uses.
- `snap.go:snapPawn` and `model/shape.ts:snapPawn` branch on `grid.type`. On a
  hex grid: creatures snap to the hex centre regardless of size, objects stay
  free, `SnapHalfCells` behaves as `SnapCells`.
- `model/grid.ts:feetMoved`/`feetBetween` gain the hex path and the unit;
  `distanceLabel` takes the unit instead of hardcoding `" ft."`.

## Phase 2: seeing it

### Tests first

- `server/js/room/render/glyphs.test.ts`: the atlas covers every character
  `distanceLabel` can emit for all four units. `GLYPHS` is currently
  `"0123456789 ft."` and silently drops anything else, so a mile reads as a blank.
- A render test that the grid pass is asked for the right type — the shader
  itself is verified by eye.
- `templ/pages/room-table_test.go`, where the grid form's tests live and
  where scenes phase 1 leaves the split ones: the *Grid* form offers the three
  types and the four units, and the Diagonals fieldset carries the variant that
  hides it on a hex type.

### Then

- `shaders/grid.ts` gains a `u_type` uniform and a hex branch: world point to
  axial, distance to the nearest of the three hex edge directions, same
  `fwidth()` antialiasing, same dash and the same zoom fade. One program, one
  uniform, no second pass.
- `GLYPHS` grows `mi`, `km` and `hex`.
- `templ/pages/room-grid.templ` — by now the *Grid* window that scenes phase 1
  split out of *Grid & settings* — gains a grid-type radio group and a unit
  beside the distance-per-cell field. The form does not redraw itself on its
  own save (`TestTheGridFormDoesNotRedrawItselfOnItsOwnSave` forbids it), so
  hiding Diagonals is CSS, not a round trip: the form is a `group` and the
  Diagonals fieldset carries `group-has-[input[value^=hex]:checked]:hidden`.
  Both names are in the `.templ`, so Tailwind emits them.
- `modes/measure.ts` and `modes/ruler.ts` count hexes along `hexLine`.

## Phase 3: the GM's map and the players' map

One layer, one grid, one set of fog and tiles and strokes, two pictures.

### Tests first

- `server/internal/room/projection_test.go`: a layer with a GM map projects to a
  player **with `gmMap` nil**, and the projected JSON contains neither the GM
  asset's id nor its generation anywhere. Assert on the marshalled bytes, not on
  the struct — this is the test that stops a future field from carrying the id
  back out by another route.
- `derive_test.go`: changing only the GM map produces a `layers.updated` for the
  GM and nothing at all for players.
- A layer with a GM map and no player map projects to players as a layer with no
  map, and the room still renders.

### Then

- `Layer` gains `GMMap *MapRef` (`json:"gmMap"`). The rule is one line in two
  places: **the GM sees `GMMap` if it is set, otherwise `Map`. Players always see
  `Map`.** Every existing room keeps its single map as the players' map and needs
  no migration.
- `project.go` clears `GMMap` on every layer for non-GM roles. `Derive` compares
  projections, so nothing else changes.
- `render/layers.ts`, which builds the `Painted` list the tile stage draws,
  picks `layer.gmMap ?? layer.map`. It needs no role check: the projection is
  what makes that correct, and `stages/tiles.ts` is untouched.
- The layer window (`room-layers.templ`) and its map picker
  (`room-maps.templ`, `RoomMaps`) gain a second slot per layer, labelled so the
  distinction is unmissable: *Players see* and *You see*. `TableSetLayerMap`
  gains a `GM bool` saying which slot it sets.

## Phase 4: fog by the cell

### Tests first

- `server/js/room/modes/fog.test.ts`: with the cell brush on a square grid a
  press-drag across three cells produces three rect shapes aligned to the grid,
  not one loose rectangle. On a hex grid it produces three six-point polys whose
  points are `hexCorners`.
- Dragging back over a cell already painted in this gesture does not send it
  twice.
- `fog_test.go`: a poly reveal of six points is accepted and counted against
  `fogBudget` as six.

### Then

- `FogOptions` gains `cells: boolean`; `fog-tool.ts` gains the toggle, rendered
  in the templ like every other tool option.
- `modes/cells.ts`: a gesture-scoped walker that turns successive pointer
  samples into the cells newly entered, using `supercover` on a square grid and
  `hexLine` on a hex one, remembering what this gesture has touched. The fog
  brush emits one shape per cell it hands back. Phase 6's stamp uses the same
  walker.
- Nothing in `internal/room` changes. This is the phase that proves rule 1: the
  fog model never learned what a hex is.

## Phase 5: the terrain shelf

Tile art is its own kind of asset with its own section, because a forest is not
a goblin. It is called *terrain* rather than *tiles* in code and on the tab
because `tiles` under `/assets/maps/{id}` already means a map's pyramid, and
`tile_size`, `tile_gen` and `map-tiles.go` all mean that too. The picture is
terrain; the stamp made from it is a tile.

### Tests first

- `server/internal/controllers/library-assets_test.go`:
  `TestTheLibraryKindsShareNothing` and
  `TestLibraryReadsAndRenamesAreScopedToTheirKind` gain the terrain kind — a
  terrain picture is not listed among tokens, a token is not listed among
  terrain, and a rename through the wrong kind's route is a 404;
  `TestAvatarsAreCroppedSquareAndTokensKeepTheirShape` gains the terrain case,
  which keeps its shape like a token; `TestALibraryPageListsOneKindForOneOwner`
  covers `/assets/terrain`.
- `templ/pages/pages_test.go`, where the tab strip is tested: five tabs,
  *Terrain* after *Maps*.
- `server/internal/hub/library_test.go`: `Picture` with `PictureTerrain` reads
  a terrain asset and refuses a token by the same id, through `assetType`.

### Then

- `db/migrations/20260912xxxxxx_terrain_assets.sql`, in the idiom
  `20260909120000_account_avatar.sql` set: `ALTER TABLE assets MODIFY type
  ENUM(..., 'terrain')`; down deletes the terrain rows and narrows the enum.
  `db/schema.sql` follows, and `make sqlc` yields
  `queries.AssetsTypeTerrain`. **No new query**: `GetLibraryAssets`,
  `SearchLibraryAssets`, `GetLibraryAsset` and `ReplaceLibraryAsset` all take
  the type as a parameter already.
- `internal/controllers/library-assets.go`: a third `libraryKind` value,
  `terrainKind`, with `Slug: "terrain"`, `One: "terrain picture"`,
  `storage.TerrainKey` beside `TokenKey`, and `images.Fit(src, terrainSize)`
  at 512 — kept whole, never squared, because the mask is applied at draw
  time. `TerrainAssetsPage`, `UploadTerrain`, `ReplaceTerrain`,
  `RenameTerrain` and `DeleteTerrain` are the same one-liners the token set
  is; `asset-search.go:assetCards` gains `terrainKind.Slug`.
- `routes.go`: `GET /assets/terrain` beside the four literal page routes, and
  the collection-and-member set the two library kinds already have.
- `templ/pages/assets.go` gains `assetTabTerrain`; `asset-tabs.templ` gets the
  link after *Maps*; `assets.templ` gets `TerrainAssets` with
  `libraryUploadButton(assetTabTerrain, "Upload terrain")` and `TerrainCards`.
  Empty-state prose lives in a `.go` file like `roomMapsEmptyHeading`, not in
  the template.
- `internal/room/resolve.go`: `PictureTerrain PictureKind = "terrain"`;
  `hub/library.go:assetType` becomes a switch over the three kinds.
- Nothing in the scenes plan changes: a terrain picture is a static image like
  a token, so `SceneLoad` never re-resolves it.

## Phase 6: tiles

The palette is the bag; a tile is a stamp in a cell; the wheel under the
pointer is how you reach into the bag.

### Tests first

- `server/internal/room/tile_test.go`:
  - stamping a cell that already holds a tile replaces it, and the state holds one
    tile for that cell, not two;
  - stamping past `TilesMax` is refused with a message that says what to do, and
    a batch past `TileBatchMax` is refused outright;
  - removing a palette entry erases every tile that references it — a GM
    cleaning up a palette should not be told no;
  - a tile references a palette entry by id, and a stamp naming an unknown entry
    is refused;
  - adding past `PaletteMax` is refused, and adding the same asset twice is
    refused;
  - a rotation that is not a multiple of 60 on a hex grid, or of 90 on a
    square one, is refused;
  - erasing is by cell, not by tile id, and a stamp records who stamped it.
- `derive_test.go` and `reduce.test.ts`: a stamp produces `tiles.stamped`, an
  erase produces `tiles.erased` carrying cells, a palette change produces
  `palette.updated`, and the TypeScript reducer replays all three against the
  regenerated fixtures.
- `server/js/room/table-menu.test.ts`, extending what scenes phase 5 leaves
  there: hovering a ring item puts its name in the hub; picking one sends
  `tiles.stamp` for the remembered cell and closes; *Erase* sends `tiles.erase`
  for it; Shift-picking arms the brush with that art instead of stamping;
  nothing in the module writes a class name.
- `server/js/room/modes/stamp.test.ts` (the brush): a press stamps the cell
  under the pointer and stays armed; a drag stamps each newly entered cell
  once; a batch flushes at `TileBatchMax` and again on release; `[` and `]`
  turn the stamp by one step; Escape puts it down; a secondary click opens the
  wheel instead of putting it down; the eraser brush sends `tiles.erase` for
  the same cells.
- `server/js/room/render/stages/order.test.ts`: `terrain` sits between `tiles`
  and `grid` for both roles.
- `server/internal/controllers/room-table-menu_test.go`: the GM's wheel carries
  one ring item per palette entry, with `--degree` stepping 360 over the count
  from the top and `--radius` at the larger of 96px and the count times 48 over
  2π, and the hub with *Erase*, *Hex note* and *Party starts here*; with an
  empty palette it is the hub alone.
- `server/internal/controllers/room-palette_test.go`: a player's `POST` to the
  palette is refused with 403; a `DELETE` naming an art nobody has is 404.
- `templ/pages/rooms_test.go`, where the room's menus are tested: the GM's
  Tabletop menu lists *Tile palette* and *Clear tiles*; a player's lists
  neither.

### Then

- State:

  ```go
  type TileArt struct {
      ID      ulid.ULID `json:"id"`
      AssetID ulid.ULID `json:"assetId"`
      Name    string    `json:"name"`
      Image   string    `json:"image"`
  }
  type Tile struct {
      LayerID  ulid.ULID `json:"layerId"`
      Art      ulid.ULID `json:"art"`
      Q        int       `json:"q"`
      R        int       `json:"r"`
      Rotation int       `json:"rotation"`
      By       ulid.ULID `json:"by"`
  }
  ```

  `Palette []TileArt` goes on `Table` **beside `Layers`, not inside
  `TableSettings`**, which `Derive` compares with `!=` and which a slice would
  make uncomparable. It is diffed with `reflect.DeepEqual` like `Layers` and
  reported as `PaletteUpdated` (`palette.updated`), which `panels.ts` maps to
  `ROOM_TABLETOP` exactly as it maps `layers.updated`. `Tiles []Tile` goes on
  `State`. `Q` and `R` are axial coordinates on a hex grid and column and row
  on a square one. **A tile has no id of its own**: `(LayerID, Q, R)` is its
  identity, which is what makes one-per-cell an invariant of the data rather
  than a rule somebody has to remember. `TileArt.ID` is scene-local so the same
  picture can sit in the bag twice under two names and so a scene body never
  depends on the library; `AssetID` is there for provenance and for the palette
  window to mark what is already in the bag. The image URL lives once per
  palette entry, not once per stamp, which is the difference between a 900-tile
  map costing ~40KB and ~120KB.
- Because tiles are not keyed by ULID they do not fit the `diff[T]` generic in
  `derive.go`. Write `cellDiff[T]` beside it, keyed by `(LayerID, Q, R)` —
  `movedPawns` and `strokeDeltas` are the precedent for a bespoke diff, and
  phase 8's notes use the same generic. Changes are `TilesStamped{Tiles}` and
  `TilesErased{Layer, Cells}`; neither maps to a UI event, because a stamp per
  drag sample must not refetch every window listening on `room:tabletop`.
  `revisions.ts` gains a `tiles` slice bumped by both.
- Commands, all wire-decodable: `PaletteAdd{Asset}` (GM; a `Resolver` reading
  `lib.Picture(ctx, asset, PictureTerrain)` for the name and URL, which refuses
  a token or an avatar by that id), `PaletteRemove{Art}`
  (GM; erases the tiles), `TilesStamp{Layer, Art, Rotation, Cells}`,
  `TilesErase{Layer, Cells}`, `TilesClear{Layer}` (GM). `Cell` is `{Q, R}`.
- Caps in `validate.go`: `TilesMax = 4_000`, `PaletteMax = 24`,
  `TileBatchMax = 64`.
- Rendering: `render/terrain-pass.ts` on the pawn program, with `SHAPE_HEX = 3`
  in `pawn-pass.ts` and the fourth branch in `shaders/pawn.ts` — a hexagon
  distance field with the same `fwidth()` edge as the disc. `stages/terrain.ts`
  goes between `tilesStage` and `gridStage` in `stages/order.ts` and its `NAMES`
  map — tiles sit on the map, under the grid lines, under the drawing, under the
  fog. It rebuilds when `revisions.tiles` moves or the viewed floor changes, the
  way `stages/strokes.ts` does, draws every tile from the sprite cache at the
  scene's cell size, and draws the stamp ghost from `overlay.stamp` last at half
  alpha. The mask is applied at draw time, so one uploaded picture works in a
  pointy-top scene and a flat-top one.
- **The wheel is the table's context menu, and the palette is its ring.**
  Right-click a cell and the menu opens centred on the pointer: a hub of small
  action buttons in the middle and one picture button per palette entry on a
  ring around it. Scenes phase 5 builds the wheel with only its hub, because
  *Party starts here* needs it first; this phase fills the ring. It is one
  surface for one gesture — there is no list menu beside it.
- The markup is a fragment, because the ring is state. `GET /fragment/room/table-menu`
  (`internal/controllers/room-table-menu.go`, from scenes phase 5) renders
  `RoomTableMenu` in `room-table-menu.templ`, and `room.templ` mounts it inside
  `#tabletop` on a hidden host that refetches on `room:tabletop`:

  ```html
  <div data-table-menu hidden class="absolute top-0 left-0 z-30" hx-get={ d.TableMenuPath() } hx-trigger="load, room:tabletop from:window" hx-swap="innerHTML"></div>
  ```

  Inside it: the hub, a `<span data-table-menu-label>` that shows the hovered
  tile's name, and a ring of
  `<button class="btn btn-circle" data-table-menu-art={ id } style={ degreeAndRadius(i, n) }><img/></button>`.
  Each ring item carries `--degree` and `--radius` inline from the server —
  `i × 360 / n` from the top and `max(96px, n × 48px / 2π)` — and sits
  `absolute` at the wheel's centre with a negative margin of half its own size,
  so that one arbitrary-property class in the `.templ`,
  `[transform:translate(calc(cos(var(--degree))*var(--radius)),calc(sin(var(--degree))*-1*var(--radius)))]`,
  is the only thing that moves it. The hub items use the same class on a
  smaller radius. The wheel is laid out by the browser, and the script never
  computes a position but the wheel's own. The hub holds *Erase* for anyone
  who may stamp, *Hex note* for the GM (and for a player once the cell's note
  is revealed, which the script decides from the store), and *Party starts
  here* for the GM. A GM with an empty palette gets the hub alone; a player
  with nothing to do gets no wheel, and `secondary` returns `false` as it does
  today.
- `table-menu.ts` (`mountTableMenu(mount, deps)`) is the script, modelled on
  `pawn-menu.ts`: `select.ts` calls `deps.tableMenu(map, screen)` from
  `secondary` over empty ground, and the module remembers the cell under the
  map point, outlines it through `overlay.cells` while the menu is open, shows
  the host at the pointer, shifts it inward by the wheel's radius where the
  pointer is nearer an edge than that, and closes on a pick, Escape, an outside
  press, a scroll, or a swap inside the host. A ring pick sends `tiles.stamp`
  for the remembered cell at rotation zero and closes. *Erase* sends
  `tiles.erase` for it. Holding Shift while picking either arms the brush
  instead. Hovering a ring item sets the hub label's text. Every class is in
  the template; the script sets text, `hidden` and the host's transform.
- The brush is `modes/stamp.ts`, modelled on `modes/place.ts` and wired like
  it: `createStamp` in `modes/table.ts`, routed by `modes/switch.ts` whenever
  it is armed, in `every` so it hovers and contributes, and arming either of
  stamp and place disarms the other. `arm` takes an art, or the eraser, or
  nothing. `press` stamps the cell under the pointer; `drag` hands each newly
  entered cell from the phase 4 walker to a batch that flushes at
  `TileBatchMax` and on release; `[` and `]` turn the stamp by 60° on a hex
  grid and 90° on a square one; Escape puts it down; a secondary click opens
  the wheel again rather than putting it down, so the wheel is how you switch
  tiles mid-paint. `contribute` writes the ghost. It is the GM's way to lay a
  region; a player crossing the map right-clicks one hex at a time and never
  needs it. It is not a `Mode`, it has no tool-bar button and no key, and
  `tools.ts` does not change.
- The GM manages the bag in a window: Tabletop menu *Tile palette*
  (`ID: "palette"`, `URL: /fragment/room/palette`, 320×420), which shows the
  entries with a *Remove* behind `hx-confirm` that says the stamped tiles go
  with it, and a search box over the terrain shelf — `SearchLibraryAssets`
  with the terrain type, the way the spawn dialog searches tokens — each result
  a button that `hx-post`s the asset to
  `POST /rooms/{id}/palette`, marked if it is already in the bag. The window
  refetches on `room:tabletop`, so the add shows up the same way every other
  table change does. `DELETE /rooms/{id}/palette/{art}` removes;
  `POST /rooms/{id}/tiles/clear` is the Tabletop menu's *Clear tiles*, layered
  and confirmed like *Clear drawing*. All three dispatch the commands above
  through the hub, in `internal/controllers/room-palette.go`. No script.

## Phase 7: players stamp

### Tests first

- `authorize_test.go`: with `PlayersCanStamp` off a player's stamp is refused
  with the same shape of message `PlayersCanDraw` gives; with it on the stamp
  lands.
- A player may erase a tile they stamped and may not erase the GM's; the GM may
  erase anything. This mirrors `StrokeErase` exactly.
- A player's stamp naming a palette entry that is not in the scene is refused.
- `room-table-menu_test.go`: a player's wheel carries the ring and *Erase*
  while the setting is on and neither while it is off, and never *Party starts
  here*; the palette window's route answers a player with 403.
- `templ/pages/room-table_test.go`: the *Table settings* form offers the
  toggle, and the *Grid* form does not.

### Then

- `TableSettings` and `TableSetOptions` gain `PlayersCanStamp`, defaulting off.
  It is a table option in the sense scenes phase 1 draws — how the GM runs the
  table, not what is on it — so it sits in the *Table settings* window with
  *Players may draw*, and stays with the room across scenes.
- `TilesStamp.Authorize` checks it the way `StrokeBegin` checks
  `PlayersCanDraw`; `TilesErase.Authorize` lets the GM erase anything and a
  player only tiles whose `By` is them, through a `requireOwnTile` shaped like
  `requireOwnStroke`.
- `GET /fragment/room/table-menu` serves a player the ring and *Erase* while
  the toggle is on and leaves them out while it is off. A player's ring is the
  GM's ring: the same entries in the same order. Only the palette window is the
  GM's, because only the Tabletop menu of a GM lists it, and only the GM's hub
  carries *Party starts here*.

## Phase 8: the hex key

### Tests first

- `projection_test.go`: an unrevealed note projects to players with **no title
  and no body** — assert on marshalled bytes again — and a revealed one projects
  whole. A note's marker projects either way, so players see that a hex has
  something in it only once it is revealed.
- `validate_test.go`: `noteBudget` refuses a note that would push the room past
  `NoteBytesBudget`, and a single note past `NoteBodyLimit`.
- `server/internal/controllers/room-notes_test.go`: the note fragment answers
  the GM always and a player only for a revealed note, and answers a player's
  request for an unrevealed one with a bare 404 and an empty body.

### Then

- `State.Notes []HexNote{LayerID, Q, R, Title, Body, Revealed}`, keyed by cell
  like tiles and diffed by the same `cellDiff[T]` into `notes.upserted` and
  `notes.removed`. Commands `NoteSet`, `NoteRemove`, `NoteReveal` (GM only, all
  three).
- `NoteBodyLimit = 4_000`, `NotesMax = 500`, `NoteBytesBudget = 200_000` — the
  same idiom as `fogBudget` and `strokeBudget`, and the reason the snapshot
  cannot grow without bound.
- `GET /fragment/room/hex?layer=&q=&r=` in `internal/controllers/room-notes.go`
  renders the note through `internal/markdown`, in a window whose id takes a
  suffix (`hex:{layer}:{q}:{r}`) the way stat blocks do. The GM's version is
  the editor; a player's is the rendered prose and nothing else.
- The key tool is not a tool: *Hex note* is a hub item on the wheel
  `plans/scenes.md` phase 5 introduces, and picking it calls `openWindow` from
  `window.ts` with the suffixed id and the fragment URL for the remembered
  cell, the way a menu item's `RoomWindow` does. If this plan runs first,
  build the wheel here — `modes/select.ts` returns `false` from `secondary`
  over empty ground today, so there is nothing to hang an item on until one
  exists.
- A cell with a note draws a marker through `overlay.cells` in the
  `floor-marks` stage, which draws tinted cell outlines today and needs no new
  pass; for players only once revealed.

## Phase 9: reveal as the party moves

### Tests first

- `pawn_test.go`: moving the party pawn into a cell with `AutoReveal` on `cell`
  adds one reveal shape for that cell and nothing else; on `neighbours` it adds
  the cell and its ring — six on a hex grid, eight on a square one; with it off
  it adds nothing; on a floor without `FogEnabled` it adds nothing.
- Re-entering a revealed cell adds no second shape.
- Only the pawn marked as the party triggers it, and marking a second pawn
  unmarks the first.
- A reveal the fog budget cannot afford is skipped and the move still lands.
- The regenerated fixtures show the move's frame carrying `pawns.moved` and
  `fog.upserted` together.

### Then

- `Pawn.Party bool` and a GM-only `PawnSetParty{ID}` that clears the flag on
  every other pawn; *This is the party* on the pawn context menu. The mark
  travels with the pawn, so it is in the scene.
- `TableSettings.AutoReveal` of `off | cell | neighbours`, an enum in the
  `Values()`/`Valid()` idiom, in `TableSetOptions` and the *Table settings*
  window. It is how the GM runs the table, so it stays with the room.
- The reveal happens **inside `PawnMove.Apply`**, the one command that moves
  pawns: after the move, for the anchor and each of `Others` that carries the
  mark, on a floor with `FogEnabled`, it appends a `poly` (hex) or `rect`
  (square) reveal for the cell and, on `neighbours`, its ring, skipping any
  whose points exactly match a reveal already there, and skipping the lot when
  `fogBudget` says no. `Derive` reports the new shapes as `fog.upserted` in the
  same frame as the move, so the client learns both at once; the fog tool lists
  and removes them like any other shape. Nothing in `hub/effects.go`, which
  runs after the fact and cannot dispatch.

---

## Where this meets the scenes plan

- **`ExportScene` and `ImportScene` grow with this plan.** `Table.Palette`,
  `Tiles` and `Notes` are scene content and join the lists in scenes phase 0;
  `Layer.GMMap` rides with the layers already. Export zeroes `Tile.By` as it
  zeroes `Stroke.By`, and a zero `By` means only the GM may erase that tile.
  When scenes phase 0 has already landed, that join is an edit to `scene.go`
  and `scene_test.go` inside this plan's phases 6 and 8, tests first.
- **`SceneLoad.Resolve` re-resolves `GMMap` exactly as it re-resolves `Map`.**
  Same `lib.Map`, same clearing on a `*room.Error`, same line in the toast.
- **`Type` and `Units` are grid and travel with the scene; `PlayersCanStamp`
  and `AutoReveal` are table options and stay with the room.** The split scenes
  phase 1 draws in the UI is the split this plan follows in the structs.
- **`Pawn.Party` is lost when the marked pawn is a player's**, because export
  drops player pawns. A hex crawl's party marker is an object or an NPC, which
  survive.

## What this deliberately does not do

- **No travel, weather, foraging or encounter mechanics.** See the top.
- **No hex coordinate labels drawn on the grid.** Rendering `0412` in every hex
  needs the glyph atlas in a new pass at a new scale, and a GM who needs
  coordinates has a keyed map that already carries them.
- **No tile layering.** One tile per cell; the map underneath is the base. Two
  tiles in a cell is a compositing question that a stamped forest over a stamped
  hill does not repay.
- **No masking at upload.** Terrain pictures are stored the way tokens are —
  `images.Fit(src, 512)` with `width`/`height` recorded, under their own type —
  and are masked and scaled at draw time to whatever the scene's hex size is.
  Baking a size makes a tile soft at any other one; baking a mask makes it
  wrong in the other orientation.
- **No tool-bar button for stamping.** The wheel under the pointer is the
  whole surface, as the spawn dialog is for placing.
- **No second ring.** Twenty-four is the cap for one ring; a palette that
  needs more is a scene that wants splitting.
