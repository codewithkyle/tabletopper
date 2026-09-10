# Phase 7: fog of war

Read `plans/vtt-overview.md` (Fog of war under Table features, and the
still-open item on server-side fog projection), `plans/phase-2-protocol-core.md`
for the fog commands, `plans/phase-4-renderer.md` for the grid pass this
borrows its shape from, and `plans/phase-5-pawns.md` for the pill, the `Tool`
contract and the reworks that shaped both. This phase hides the map: the GM
covers a floor and cuts reveals out of it, and the players see only what has
been uncovered.

It was the first quarter of a plan that also held strokes, pings and
initiative, split on 2026-09-09 after phase 5 shipped. Phase 6 is initiative
and phase 8 is strokes and pings. Nothing here was built before the split.

**Revised 2026-09-09** against the GM's own description of the feature. Four
decisions moved and they are marked below: the prefill is one room-wide switch
in Grid & settings rather than a radio per floor, `Fill fog` and `Clear fog`
stay as menu items rather than becoming rows of a window, the polygon closes on
the right button, and there is no Fog window at all.

## Already built

- `internal/room/fog.go`: `FogShape{ID, LayerID, Kind, Mode, Points}` with
  `Kind` one of `rect | poly` and `Mode` one of `reveal | hide`. Shapes apply
  in slice order and `Normalize` leaves the order alone. A layer carries
  `FogEnabled` (false by default) and `FogPrefill` (true by default, and a
  new layer inherits nothing from the one below). `fog.setEnabled` and
  `fog.setPrefill` end in `table.updated`; `fog.add` is the GM's, and a
  rectangle is exactly two corners while a polygon is at least three points,
  under `FogPointsMax` of 2,000 numbers, `FogShapesMax` of 2,000 shapes and
  `FogPointsBudget` of 200,000 coordinates; `fog.remove` names one shape;
  `fog.clear` names a layer. `table.removeLayer` and `table.clear` emit
  `fog.cleared` for every layer they empty.
- All five commands are already in `command.go`'s socket table, so nothing
  below needs a new message type.
- The reducer on both sides applies all five events. Players hold every
  floor's shapes, not only the active one, which the overview records as
  still open.
- The pill has five buttons -- Select, Move, Measure, Fog, Draw. `tools.ts`
  finds what a button DOES by `data-room-tool-pans` and
  `data-room-tool-measures`, and its letter by `data-room-tool-key`; Fog
  carries neither, so choosing it leaves the table behaving as Select.
- `render/input.ts` has the `Tool` contract -- `press`, `drag`, `release`,
  `cancel`, `secondary`, `hover`, `active` -- and `pawns.ts` implements it,
  asking `deps.panning()`, then placement, then `deps.measuring()`, then the
  handles, then the hit test. `abandon()` is what Escape and the right button
  call, and `keys.ts` says which keys are the table's.
- The renderer draws, in order: tiles, grid, blood, the ruler's cells, pawns,
  condition rings and outlines, ghosts, handles, the ruler's line and label.
  `grid-pass.ts` draws one full-viewport triangle and maps every pixel back to
  map space through `inverseClipMatrix`; `gl.ts` has `fullscreenTriangle`,
  `createProgram` and `uniforms`. The clear colour is read from the mount's
  computed background through a one-pixel 2D canvas and re-read on
  `theme:change`. `layers.ts` settles the viewed floor and crossfades maps.
  `ring-pass.ts` draws rectangles and ellipses; `path-pass.ts` draws lines.
- `hub.Table` answers `TableView{Table, Pawns}` and loads the room. The layer
  manager is a window on `room:tabletop`, and `layerCommand` is the helper
  its mutations share.
- The GM's bar has a Fog menu with two disabled lines, `Fill fog` and
  `Clear fog`, from the old client.
- `Grid & settings` is `pages/room-grid.templ` posting to `SetRoomGrid`, which
  dispatches `table.setGrid` and then `table.setOptions`.
- `data-room-action` is the established shape for a menu item that acts on
  client state: `room.js` reads it and raises a window event the room bundle
  listens for. `Clear blood` and every View item work that way.
- `layer-bar.ts` records why a path cannot be rewritten on an htmx element
  after processing, and reaches for `htmx.ajax` instead.
- vanilla-colorful is already a runtime dependency bundled into `room.js`.

Not built: any fog rendering, the fog tool, a `room:fog` event, the room-wide
prefill flag, and the two menu routes.

## End state

- Grid & settings carries one more switch, `Prefill fog`, off by default. It
  is the room's answer to "does a new floor start covered", and nothing else.
- The GM chooses Fog in the pill -- the button is theirs alone and the key is
  `F` -- and a second pill appears under the first with Rectangle or Polygon
  and Reveal or Hide. A rectangle is dragged and lands on the button coming
  up. A polygon is clicked corner by corner and closed with the right button
  or Enter. Escape abandons either, the right button abandons a rectangle,
  Backspace takes a polygon's last corner back, and Ctrl+Z removes the newest
  shape on the floor being viewed.
- The Fog menu's two lines work: `Fill fog` covers the floor the GM is looking
  at and `Clear fog` uncovers it, both behind the confirm modal, and both act
  on the VIEWED floor rather than the active one.
- Players see the floor's map under a cover in the table's own colour, with
  holes where the GM has cleared; a pawn under the cover is not drawn, not
  labelled and not selectable for them, unless it is their own. The GM sees
  the same cover as a tint under the pawns.
- Fog and the map crossfade together when the GM changes floor, and a
  reconnect or a snapshot rebuilds the cover exactly.

## Decisions

1. **Fog shapes are the source of truth and the mask is a cache.** Each client
   rasterises the viewed floor's shapes into an offscreen `R8` texture in a
   framebuffer, at a quarter of the mask rectangle's size and never more than
   4096 on the long side. A new shape on the viewed floor is drawn onto the
   existing mask; a removal, a clear, a prefill change, a change of viewed
   floor or of the mask's rectangle rebuilds it. Polygons are triangulated by
   ear clipping in `fog.ts` and a rectangle is two triangles.

   *Changed.* The first draft reached for `earcut` as the room bundle's second
   runtime dependency. Sixty lines is the whole of what these polygons need --
   a simple ring a GM clicked out, no holes, at most a thousand corners under
   `FogPointsMax` -- and `path.ts` already writes its own supercover
   rasteriser rather than taking a dependency for the same reason. What the
   library would have bought is graceful behaviour on a ring that crosses
   itself, which a GM can draw; `triangulate` answers that with a bail-out and
   a fan, and a test pins that it terminates.
2. **The cover is a full-viewport triangle, not a map-sized quad.** Phase 4
   made the grid infinite because play leaves the image -- a chase off the
   north road, a camp in the woods -- and a cover the size of the map would
   leave a pawn standing off its edge in plain sight on a prefilled floor. So
   the pass is the grid pass's shape: one triangle, each pixel mapped back to
   map space through the inverse camera. Inside the mask's rectangle it
   samples the mask; outside it, it uses the floor's prefill.
3. **The mask's rectangle is the map's rectangle UNIONED with the shapes'.**
   *Changed.* The first draft sized the mask to the map and fell back to the
   shapes' bounding box only for a floor with no map, which would have dropped
   any shape drawn past the map's edge -- and the tool allows exactly that,
   because decision 2 makes the cover infinite and a GM clearing the road out
   of town is clearing ground the image does not cover. So the rectangle is
   the union of the map's rectangle, when there is one, and the bounding box
   of the floor's shapes, padded by a cell; a change to that union is a
   rebuild. A floor with neither is a rectangle of nothing and the shader
   never samples it.
4. **The fog is the table's own colour, read the way the clear colour already
   is.** A player's cover is that colour at full alpha, so a covered floor is
   indistinguishable from the empty desk around the map and there is no map
   edge to read; the GM's tint is the same colour at half alpha over the map,
   which is the old client's mix and reads as "hidden from them" rather than
   as damage to the picture. Both come from the probe `readClearColor`
   already runs and both re-read on `theme:change`, so caramellatte, coffee
   and the system default all follow -- which the old client did not do: it
   hard-coded 0.98 grey and black off `prefers-color-scheme`.
5. **Where it is drawn depends on who is looking.** For a player the cover
   goes after the ghosts and the handles and before the ruler's line and
   label, so it hides the map, the grid, the blood, the pawns and the rings,
   and the player's own ruler still reads over it. For the GM the tint goes
   after the blood and before the ruler's cells and the pawns, so what is
   hidden from the party is tinted under the creatures the GM is moving
   through it.
6. **Concealment is client-side and it gates three things for a player.** The
   draw, the hover label and the hit test, and the marquee, all through one
   `covered(x, y)` in `fog.ts` that walks the viewed floor's shapes in order
   from the prefill. **A player's own pawns are never concealed from them**, so
   somebody walking into an unlit room does not watch their own token vanish.
   The data has still reached the browser; server-side fog-aware projection
   stays deferred in the overview, and it would be half a measure while the
   map is a URL a player can open. **Fog is presentation, not secrecy**, and
   this phase makes the leak a devtools tab rather than a screen.
7. **The Fog tool is the pill's fourth button and it is the GM's.** `RoomTool`
   gains `Fogs` and `GM`; `RoomTools()` gives Fog both and the key `f`; the
   template renders a `GM` tool only for the GM, so a player's pill has four
   buttons and no dead one. `tools.ts` finds it by `data-room-tool-fogs`, the
   way it finds the other two, and `TestExactlyOneToolIsTheRuler` gains its
   fog counterpart.
8. **The tool's options are a second pill, and it goes UNDER the first.**
   Shape -- Rectangle or Polygon -- and mode -- Reveal or Hide -- are the
   tool's own state, kept in `fog-tool.ts` and shown as four `aria-pressed`
   buttons rendered in templ, `hidden` unless Fog is the chosen tool. It is
   under rather than beside because the main pill is vertical at the top right
   and its tooltips open to the LEFT, which is where a second pill beside it
   would sit. Phase 8 gives Draw one of the same shape for colour and width.
9. **Corners snap to the grid's vertices unless Alt is held.** A fog reveal is
   a room, and rooms are drawn on the grid; snapping the corners to cell
   vertices gives clean edges with no effort, and Alt is the bypass for a
   diagonal corridor. Snapping is off when the grid's snap is `off`.

   **INTEGERS ARE WHAT GO ON THE WIRE AND `snapCorner` IS WHERE THAT IS MADE
   TRUE.** A corner arrives as map pixels off `screenToWorld`, which is a
   float; `FogAdd.Points` is `[]int`, and `encoding/json` refuses a fraction
   outright rather than truncating it. `snapAxis` rounds in its two snapping
   modes and hands the value straight back in `off`, so a room with snapping
   switched off answered "Bad command" to every shape until `snapCorner`
   rounded on every path. The rounding is here rather than in `snapAxis`,
   whose other caller places a pawn and wants the unrounded answer.
10. **The first shape on a floor whose fog is off turns the fog on AND sets the
    prefill to match the mode, IN `FogAdd` ITSELF.** *Changed.* Drawing on a
    floor with fog disabled would otherwise do nothing visible, and nothing on
    the screen would say why. A Reveal is a hole and a hole means nothing
    except in something solid, so the floor becomes covered; a Hide is a patch
    and a patch means nothing except on something clear, so the floor becomes
    clear. The first draft had the client send `fog.setEnabled` and
    `fog.setPrefill` ahead of `fog.add`; doing it in the command is one message
    instead of three, one ordering instead of three, and a second GM's browser
    learns the flag and the shape in the same breath. `FogAdd` emits
    `table.updated` before `fog.added` on the one add that wakes a floor, and
    nothing extra on every add after it.
11. **Undo is `fog.remove` of the newest shape on the floor.** Ctrl+Z in Fog
    mode sends it for the viewed floor from the store's copy of the shapes.
    The store keeps the shapes in the order they were added, so "newest" is
    the last one on that floor. It is the only repair for a mis-drawn clear
    that is not `Fill fog` and starting the floor again.
12. **The prefill is one room-wide switch and it is a DEFAULT for new floors.**
    *Changed.* `Table.FogPrefill`, false by default, rendered as one more
    toggle at the foot of Grid & settings and carried on `table.setOptions`
    beside the two options already there. `TableAddLayer` reads it: on, and a
    new floor arrives `FogEnabled: true, FogPrefill: true`, which is a floor
    that is covered the moment it exists; off, and it arrives as it does today.
    It changes NO existing floor, because a switch that covered five floors at
    once would throw away an evening of clearing on four of them, and the
    per-floor answer is two menu items away.
13. **`Fill fog` and `Clear fog` stay menu items and act on the VIEWED floor.**
    *Changed.* The first draft replaced both with a window holding a row per
    floor. Two lines that already exist and already say what they do are a
    better fit than a window, and the floor they mean is the one the GM is
    looking at -- a GM covering the first floor while the party is in the
    cellar is the case the feature is for. Which floor that is exists only on
    the client, so the layer travels as a form value that the room bundle
    keeps current, not as a path segment: `layer-bar.ts` already records why a
    path on a processed htmx element cannot be rewritten.
14. **Both menu items are destructive and both are behind the confirm modal.**
    `Fill fog` is `fog.clear` then `setPrefill(true)` then `setEnabled(true)`;
    `Clear fog` is `fog.clear` then `setEnabled(false)`. Each throws away every
    shape on that floor and neither can be undone, so each carries
    `hx-confirm`. There is no fourth dialog and no `window.confirm`.
15. **There is no Fog window and no `hub.Fog`.** *Changed.* With the prefill in
    Grid & settings and the two verbs on the menu, what the window was for is
    gone: a shape count nobody acts on, a per-floor switch that is now a
    default, and an Undo button that Ctrl+Z does better. `TableView` gains
    nothing.

## Server

### State

- `room.Table` gains `FogPrefill bool` (`json:"fogPrefill"`). Zero value false
  is the default the switch ships in, so no snapshot migration is needed:
  every existing room reads back as off.
- `room.TableSetOptions` gains the same field and `Apply` assigns it. It is a
  flag with no closed set, so there is nothing to validate.
- `TableAddLayer` seeds the new floor from it. The comment there about NOT
  inheriting from the layer below stays true and gets one more sentence: it
  inherits from the ROOM, which is a setting the GM chose once.

### Routes

| Pattern | Wrapper | Handler | Behaviour |
| --- | --- | --- | --- |
| `POST /rooms/{id}/fog/fill` | `RequireSession` | `FillLayerFog` | GM only, behind `hx-confirm`. Reads `layer` from the form. Dispatches `fog.clear`, `fog.setPrefill(true)`, `fog.setEnabled(true)`. 204. |
| `POST /rooms/{id}/fog/clear` | `RequireSession` | `ClearLayerFog` | GM only, behind `hx-confirm`. Reads `layer` from the form. Dispatches `fog.clear`, `fog.setEnabled(false)`. 204. |

Both are POST rather than one POST and one DELETE because a menu item is a
button carrying `hx-post` and a second verb would be a second branch in
`roomBarItem` for one route. **The layer is a form value and not a path
segment**, for decision 13's reason. A layer the room does not hold is the
core's refusal through `rejectCommand`, which is the alert modal; a `layer`
that is not a ULID at all is a 404 with an empty body.

Both answer 204 and redraw nothing, because both end in events the panels
already refetch on.

### Templates

- `pages/room-grid.templ`: one more `gridToggle`, `fogPrefill`, captioned
  "New floors start covered by fog". `RoomGridData` gains `FogPrefill bool`,
  `gridData` fills it and `gridForm` reads it.
- `pages/room.go`: the Fog menu's two items become live. Both carry `Post`,
  `Confirm`, `ConfirmHeading` and `ConfirmLabel`; `Clear fog` carries
  `Danger`. `RoomMenuItem` gains `Layered bool`, which renders
  `data-room-layered` and an empty `hx-vals` for the room bundle to fill.
- `pages/room.go`: `RoomTool` gains `Fogs` and `GM`; `RoomTools()` gives Fog
  `Fogs: true, GM: true, Key: "f"`; `RoomPageData.Tools()` filters the GM ones
  out for a player.
- `pages/room.templ`: `roomToolbar` renders `data-room-tool-fogs`, and a new
  `roomToolOptions()` under the pill -- `data-room-tool-options`, `hidden`,
  four buttons carrying `data-fog-shape="rect|poly"` and
  `data-fog-mode="reveal|hide"` with `aria-pressed`, an icon each and a
  tooltip. The polygon's tooltip is where "Right click or Enter closes it" is
  said.
- Words that must not appear in the new markup as attribute names, values or
  ids: `mask`, `filter`, `join`, `swap`, `list`, `table`, `visible`. The mask
  is only ever a `mask` in TypeScript, which is not scanned. Run the selector
  diff after every change under `server/templ`.

## Client

### Modules

```
render/fog-pass.ts   the mask framebuffer, shape rasterisation, the cover and the tint
fog.ts               shape geometry, triangulation, covered(), snapping, the gesture in hand
fog-tool.ts          the options pill: shape and mode, hidden while another tool is chosen
fog-menu.ts          the two menu items' hx-vals, kept on the viewed floor
```

### The mask

`fog-pass.ts` keeps one `R8` texture in a framebuffer, the floor id, the
rectangle, the prefill and the list of shape ids it has drawn. `sync(shapes,
floorID, map, prefill, cell)` runs once per frame before the draw.

**It computes the rectangle rather than being handed one.** *Changed.* The
first draft had the renderer call `maskRect` per frame and pass the answer in,
which walks every coordinate on the floor -- two hundred thousand of them
under `FogPointsBudget` -- to produce the same rectangle it produced last
time. So the first thing `sync` does is build a signature out of the floor,
the prefill, the shape count, the newest shape's id, the map's size and the
cell, and return when it has not changed. Ids are minted in order and a new
shape is appended, so that pair catches every add, every removal and every
clear. `maskRect` runs only past that gate.

Past it: when the floor, the rectangle or the prefill differs, or the drawn
ids are not a prefix of the floor's shapes in order, it clears the mask to the
prefill's value and draws every shape; when the drawn ids are a proper prefix,
it draws the new ones onto what is there. A reveal writes 1 and a hide writes
0 into the red channel; a covered prefill clears to 0 and a clear prefill to 1.

The rasteriser is one program taking map-space triangles and the rectangle,
drawn with `TRIANGLES` from a buffer that grows by doubling, **one draw call
per run of shapes sharing a mode** -- which for a floor of nothing but reveals
is one call however many there are.

The cover is the grid pass's shape with a different fragment shader: the
inverse clip matrix, the mask's rectangle, the mask sampler, the prefill and
the colour with its alpha. Inside the rectangle the shader samples the mask
with `LINEAR`, so a quarter-resolution edge is soft rather than stepped, and
outputs the colour where the sample is under a half; outside it, it outputs
the colour when the prefill is covered. `draw(role, ...)` is called at the
position decision 5 gives.

### The tool

`fog.ts` exports `createFog(deps)` with `press(map, mods)`, `drag(map, mods)`,
`release(map, mods)`, `secondary()`, `hover(map)`, `key(e): boolean`,
`abandon(): boolean`, `outline()`, `marks(out)`, `covered(x, y)` and
`concealed(pawn)`.

**The rectangle preview is `outline()` and it is ONE outline.** `Table.outlines`
fills reused slots and truncates to a count, so a fog accessor that pushed onto
the array would have its work thrown away by the truncation -- which is exactly
what shipped first: `outlines(out)` existed, nothing called it, and the tool
worked while drawing nothing. There is only ever one rectangle in hand, so it is
a single reused object added through the same helper the marquee uses.

**There is no `active()`.** A half-drawn polygon does not keep the frame loop
awake: the rubber band follows the POINTER, every pointermove asks for a frame
of its own, and a GM who stops to think about the next corner would otherwise
leave the tab rendering at sixty frames a second until they made up their mind.

**`covered()` walks the floor's shapes BACKWARDS and stops at the first hit**,
which is the same answer -- the last shape containing the point decides -- at a
fraction of the cost, and every shape carries a bounding box cached in a
`WeakMap` to reject against first. It is asked once per pawn on every hover,
every press and every marquee, so a polygon's fifty coordinates cannot be on
that path.
`pawns.ts` asks `deps.fogging()` after placement and before measuring in
`press`, and hands the gesture over for as long as the tool is chosen;
`abandon()` asks the fog first after the armed check, so Escape puts a
half-drawn polygon away exactly as it puts a ruler away. The Enter, Backspace
and Ctrl+Z keys go to `key()` from the document listener `pawns.ts` already
has, behind `typing()`, and only while the tool is chosen.

- Rectangle: press records the first corner, drag moves the second and the
  preview is one `RING_RECT` outline through `table.outlines`; release with the
  two snapped corners differing on both axes sends `fog.add` with them
  normalised to minimum then maximum, and anything that snaps to nothing is a
  click and sends nothing. *Changed:* the test is on the SNAPPED corners rather
  than on a cell of hand movement, because that is what decides whether the
  shape has any area -- and it lets a half-cell rectangle exist under
  `halfCells` snapping, which a cell threshold would have refused.
  **Both preview colours are light** and differ by hue, not by lightness: a
  dark outline over a dark dungeon is an outline nobody can aim with.
  **The right button abandons it**, which is what the right button already
  means everywhere else on this table.
- Polygon: each press adds a snapped corner; `lines(out)` gives the renderer
  the segments between them and a rubber band to the pointer, drawn through
  the over-marks path pass. **The right button closes it** with three or more
  corners, drawing the last edge back to the first, and Enter does the same --
  the right button because that is the gesture the GM asked for and every
  mapping tool has, Enter because the old client taught it and it costs one
  line. Backspace takes the last corner back; the last corner taken back is an
  empty polygon and no gesture. Escape drops the lot.
- Both send the viewed floor and integers, and nothing else: waking a floor
  whose fog is off is the server's, per decision 10. `covered()` reads the
  store and the viewed floor and is asked by `pawns.ts` through a
  `concealed(pawn)` dependency that is always false for the GM and false for a
  pawn this player owns.

**The right button means two things inside one tool and that is deliberate.**
It commits a polygon, which has something half-made to commit, and it abandons
a rectangle, which does not. `pawns.ts` routes `secondary` to the fog tool
first and falls through to `abandon` when the fog has nothing in hand.

`fog-tool.ts` mounts the options pill: it reads the four buttons, keeps
`shape` and `mode`, sets `aria-pressed`, and hides or shows the pill from
`tools.onChange` by asking `tools.fogging()`. `tools.ts` grows `fogging()`
beside `panning()` and `measuring()`, asked of the CHOSEN button for the
reason `measuring()` is: the space bar borrows the pointer for the camera and
does not put the polygon in hand away.

`fog-menu.ts` finds `[data-room-layered]` and writes `hx-vals` with the viewed
floor's id, on mount and on every `renderer.onSettled`. It writes no class
name and no path: the path is the template's and only the value moves.

### The renderer

`drawFrame` calls `fog.sync(...)` with the viewed floor's shapes, its
rectangle and its prefill after `layers.update`, and `fog.draw(...)` at the
role's position. The GM's viewed floor and a player's active floor both come
from `layers.viewed()`, which is already the floor everything else draws.
During a crossfade the mask is the incoming floor's from the first frame; a
fade of the fog itself is not worth a second mask.

`renderer.invalidate()` already runs on every event, and the sync per frame is
what notices the shapes changed. Player-side hit testing, hovering and the
marquee filter through `concealed`, so a pawn under the cover is not found by
any of them.

**`touchesPawns` gains the fog family, and that is not optional.** The pawn
instance buffer is rebuilt on a change rather than per frame, and concealment
decides what goes INTO it -- so uncovering a room changes the buffer without
changing a single pawn. Without the fog events in that predicate a player
watches an uncovered room stay empty until the next time anybody moves anything,
which is what shipped first: `table.updated` covered the two flags, so the
FIRST shape on a sleeping floor looked right and every one after it did not.
It moved from `main.ts` to `effects.ts` to be testable at all; `main.ts` reads
the document at module scope.

## Tests

**TypeScript**:

- `fog.test.ts`: a rectangle produces two triangles in map coordinates with
  corners normalised; a concave polygon triangulates with no degenerate
  triangle; `covered()` answers the prefill for a point in no shape, a reveal
  over a covered floor, a hide drawn over that reveal, and ignores a shape on
  another floor; corners snap to vertices under `cells` and `halfCells`, not
  under `off`, and not with Alt; a rectangle smaller than a cell is not sent;
  the mask rectangle is the union of the map's and the shapes'.
- `pawns.test.ts` gains: a fog rectangle sends exactly one `fog.add` with the
  snapped, normalised corners; Escape mid-polygon sends nothing; the right button
  mid-polygon with two corners sends nothing and with three sends one
  `fog.add`; the right button mid-rectangle sends nothing; a player's hover,
  press and marquee skip a concealed pawn and skip neither their own nor one
  on a cleared patch.
- `pawns.test.ts` also pins the rectangle preview: that a drag produces one
  outline centred between its snapped corners, that a drag that has not left
  its first vertex produces none, that abandoning it takes it off the table,
  and that the two modes are different colours.
- `effects.test.ts` gains `touchesPawns`: the three fog events rebuild the pawn
  buffer, the pawn family and `snapshot` and `table.updated` do, and
  `pawn.dragging` does not.
- `tools.test.ts` gains nothing, and that is deliberate: it tests the pure half
  of that module -- `showing` and `typing` -- because `mountTools` is DOM-bound
  and node has no DOM. `fogging()` is `measuring()`'s twin line and is covered
  the two ways `measuring()` is: `showing` pins that a held space bar does not
  change what was CHOSEN, and `pawns.test.ts` pins that a gesture begun under
  the fog finishes under it. The template test pins that exactly one button
  carries `data-room-tool-fogs`.

**Go**:

- The two routes dispatch the right commands in the right order for the layer
  in the form; a player is refused on both with a 403 alert; a `layer` that is
  not a ULID is a 404.
- `TableSetOptions` writes `FogPrefill`, and `TableAddLayer` seeds a new
  layer's two flags from it both ways round.
- `FogAdd` on a floor with fog off turns it on, sets the prefill from the mode
  both ways round, and emits `table.updated` before `fog.added`; a second add
  on the same floor emits `fog.added` alone.
- Template tests pin that the Fog tool renders only for the GM, that exactly
  one tool carries `data-room-tool-fogs`, that both fog menu items carry
  `hx-confirm` and `data-room-layered`, and that Grid & settings carries the
  `fogPrefill` toggle.

## Verification

1. `make js && make check` and the CSS selector diff; every added selector
   should be one the options pill or the new toggle uses.
2. GM turns `Prefill fog` on, adds a floor: it arrives covered, and the floors
   that were already there are untouched.
3. GM presses `Fill fog` on the ground floor: the player's table is a solid
   block of the table colour with no map edge anywhere in it, and the GM's map
   is tinted. GM drags a rectangle reveal and clicks out a polygon reveal,
   closing it with the right button; the player sees the map there and nothing
   else, and no pawn outside the reveals. GM draws a hide polygon over part of
   a reveal: it closes back up on both screens.
4. Player hovers where a hidden goblin stands under the cover: no label.
   Player marquees across it: not selected. Player's own character walks under
   the cover: still drawn, still theirs. GM reveals the goblin: it appears.
5. Ctrl+Z in the GM's Fog mode removes the newest shape on both screens.
   `Clear fog` empties the floor behind the confirm.
6. GM views the first floor while the party is in the cellar and presses
   `Fill fog`: the FIRST floor is covered and the cellar is untouched.
7. Drag a pawn off the map's edge on a covered floor: the player does not see
   it beyond the edge either. Clear a rectangle entirely outside the map's
   bounds: it appears. Zoom out until the whole map is a stamp: the cover
   still ends where the clears end.
8. Kill and restart the server mid-fight: both clients reconnect with the fog
   exactly as it was. Change the theme: the cover follows the table colour.
9. Benchmark with the mask on and two hundred shapes; record the numbers
   against phase 5.

## Out of scope

Server-side fog-aware projection (still deferred in the overview), a brush or
eraser tool for fog, per-shape editing after the fact, a reveal that follows a
pawn's light radius, dynamic lighting and line of sight, fog on a per-player
basis, and strokes and pings, which are phase 8.
