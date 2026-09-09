# Phase 7: fog of war

Read `plans/vtt-overview.md` (Fog of war under Table features, and the
still-open item on server-side fog projection), `plans/phase-2-protocol-core.md`
for the fog commands, `plans/phase-4-renderer.md` for the grid pass this
borrows its shape from, and `plans/phase-5-pawns.md` for the pill, the `Tool`
contract and the reworks that shaped both. This phase hides the map: the GM
covers a floor and cuts reveals out of it, or leaves it clear and paints hides
onto it, and the players see only what has been uncovered.

It was the first quarter of a plan that also held strokes, pings and
initiative, split on 2026-09-09 after phase 5 shipped. Phase 6 is initiative
and phase 8 is strokes and pings. Nothing here was built before the split.

## Already built

- `internal/room/fog.go`: `FogShape{ID, LayerID, Kind, Mode, Points}` with
  `Kind` one of `rect | poly` and `Mode` one of `reveal | hide`. Shapes apply
  in slice order and `Normalize` leaves the order alone. A layer carries
  `FogEnabled` (false by default) and `FogPrefill` (true by default, and a
  new layer inherits nothing from the one below). `fog.setEnabled` and
  `fog.setPrefill` end in `table.updated`; `fog.add` is the GM's, and a
  rectangle is exactly two corners while a polygon is at least three points,
  under `FogPointsMax` of 2,000 numbers and `FogShapesMax` of 2,000 shapes;
  `fog.remove` names one shape; `fog.clear` names a layer.
  `table.removeLayer` and `table.clear` emit `fog.cleared` for every layer
  they empty.
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
- vanilla-colorful is already a runtime dependency bundled into `room.js`.

Not built: any fog rendering, the fog tool, the fog window, a `room:fog`
event, and any `TableView` field about shapes.

## End state

- The GM chooses Fog in the pill and a second pill appears beside it with
  Rectangle or Polygon and Reveal or Hide. A rectangle is dragged; a polygon
  is clicked corner by corner and closed with Enter. Escape and the right
  button abandon the shape in hand, Backspace takes the last corner back, and
  Ctrl+Z removes the newest shape on the floor being viewed. The first shape
  drawn on a floor whose fog is off turns it on.
- Players see the floor's map under a cover in the table's own colour, with
  holes where the GM has revealed; a pawn standing under the cover is not
  drawn, not labelled and not selectable for them. The GM sees the same cover
  as a tint under the pawns.
- The GM's Fog menu opens the Fog window: a row per floor with an on-off
  switch, a choice of starting covered or clear, the number of shapes, an
  Undo last button and a Clear behind the confirm modal.
- Fog and the map crossfade together when the GM changes floor, and a
  reconnect or a snapshot rebuilds the cover exactly.

## Decisions

1. **Fog shapes are the source of truth and the mask is a cache.** Each client
   rasterises the viewed floor's shapes into an offscreen `R8` texture in a
   framebuffer, at a quarter of the map's native resolution and never more
   than 4096 on the long side. A new shape on the viewed floor is drawn onto
   the existing mask; a removal, a clear, a prefill change, a change of viewed
   floor or of the mask's rectangle rebuilds it. Polygons are triangulated
   with `earcut`, pinned in `package.json` as the room bundle's second
   runtime dependency after vanilla-colorful; a rectangle is two triangles
   with no library.
2. **The cover is a full-viewport triangle, not a map-sized quad.** Phase 4
   made the grid infinite because play leaves the image -- a chase off the
   north road, a camp in the woods -- and a cover the size of the map would
   leave a pawn standing off its edge in plain sight on a prefilled floor. So
   the pass is the grid pass's shape: one triangle, each pixel mapped back to
   map space through the inverse camera. Inside the mask's rectangle it
   samples the mask; outside it, it uses the floor's prefill.
3. **The mask's rectangle is the map's, or the shapes' when there is none.**
   A floor with no map is a grid over the table colour, and a GM can still fog
   it; without an image to size the mask by, the rectangle is the bounding box
   of the floor's shapes padded by a cell, and a change to it is a rebuild.
   Which is to say the tool works on a blank floor and nobody has to special-
   case one.
4. **The fog is the table's own colour, read the way the clear colour already
   is.** A player's cover is that colour at full alpha, so a covered floor is
   indistinguishable from the empty desk around the map; the GM's tint is the
   same colour at half alpha over the map, which is the old client's mix and
   reads as "hidden from them" rather than as damage to the picture. Both
   re-read on `theme:change`.
5. **Where it is drawn depends on who is looking.** For a player the cover
   goes after the ghosts and the handles and before the ruler's line and
   label, so it hides pawns, rings, blood and previews and the player's own
   ruler still reads over it. For the GM the tint goes after the blood and
   before the ruler's cells and the pawns, so what is hidden from the party is
   tinted under the creatures the GM is moving through it.
6. **Concealment is client-side and it gates three things for a player.** The
   draw, the hover label and the hit test, and the marquee, all through one
   `covered(x, y)` in `fog.ts` that walks the viewed floor's shapes in order
   from the prefill. A player's own pawns are never concealed from them. The
   data has still reached the browser; server-side fog-aware projection stays
   deferred in the overview, and this phase makes the leak a devtools tab
   rather than a screen.
7. **The Fog tool is the pill's fourth button and it is the GM's.** `RoomTool`
   gains `Fogs` and `GM`; `RoomTools()` gives Fog both and the key `f`; the
   template renders a `GM` tool only for the GM, so a player's pill has four
   buttons and no dead one. `tools.ts` finds it by `data-room-tool-fogs`, the
   way it finds the other two, and `TestExactlyOneToolIsTheRuler` gains its
   fog counterpart.
8. **The tool's options are a second pill.** Shape -- Rectangle or Polygon --
   and mode -- Reveal or Hide -- are the tool's own state, kept in
   `fog-tool.ts` and shown as four `aria-pressed` buttons rendered in templ,
   `hidden` unless Fog is the chosen tool. It sits beside the main pill and
   phase 8 gives Draw one of the same shape for colour and width. They are
   not in the Fog window, which is a refetching fragment and would reset them.
9. **Corners snap to the grid's vertices unless Alt is held.** A fog reveal is
   a room, and rooms are drawn on the grid; snapping the corners to cell
   vertices gives clean edges with no effort, and Alt is the bypass for a
   diagonal corridor. Snapping is off when the grid's snap is `off`, and the
   snapped integers are what go on the wire.
10. **The first shape on a floor whose fog is off turns the fog on.** Drawing a
    reveal on a floor with fog disabled would otherwise do nothing visible, and
    nothing on the screen would say why. The client sends `fog.setEnabled`
    and then `fog.add`, both over the socket -- the socket carries what
    originates on the canvas, and this originates there.
11. **Undo is `fog.remove` of the newest shape on the floor.** Ctrl+Z in Fog
    mode sends it for the viewed floor from the store's copy of the shapes;
    the Fog window's Undo last button posts the same thing over HTTP, with the
    newest shape's id rendered into the button by the fragment. The store
    keeps the shapes in the order they were added, so "newest" is the last
    one on that floor.
12. **The Fog window is per room, one row per floor, and it replaces both old
    menu items.** `Fill fog` is what a prefilled floor with no shapes already
    is, so it becomes the Covered choice plus Clear; `Clear fog` is a button
    on each floor's row. It refetches on `room:tabletop` for the two flags and
    on `room:fog` for the count, which `panels.ts` raises for the three fog
    events. Nothing in it lists shapes one by one: a well-explored floor holds
    hundreds, and the one somebody wants gone is the last one.
13. **`TableView` gains the shapes' summary and there is no `hub.Fog`.** The
    window needs a count and the newest id per floor beside the flags the
    table already carries, and the layer manager already reads the table in
    one message. `TableView.Fog map[ulid.ULID]FogSummary{Count int; Newest
    ulid.ULID}` is counted on the actor's goroutine like the pawn counts.

## Server

### Routes

| Pattern | Wrapper | Handler | Behaviour |
| --- | --- | --- | --- |
| `GET /fragment/room/fog?room={id}` | `Fragment` | `RoomFogFragment` | GM only, else an empty 404. The Fog window, from `gmTable`. |
| `POST /rooms/{id}/layers/{layer}/fog` | `RequireSession` | `SetLayerFog` | GM only. Reads `enabled` (a checkbox) and `prefill` (a radio, `covered` or `clear`), compares them with the live table, and dispatches `fog.setEnabled` and `fog.setPrefill` only for the one that changed, so one click is one `table.updated`. 204. |
| `DELETE /rooms/{id}/fog/{shape}` | `RequireSession` | `RemoveFogShape` | GM only. Dispatches `fog.remove`. 204. |
| `DELETE /rooms/{id}/layers/{layer}/fog` | `RequireSession` | `ClearLayerFog` | GM only, behind `hx-confirm`. Dispatches `fog.clear`. 204. |

All three mutations go through `layerCommand`, which already parses the layer
from the path and answers a refusal with the alert modal; the shape route uses
it with the shape parsed the same way. Every one answers 204 and redraws
nothing, because every one ends in an event the window refetches on.

### Templates

- `pages/room-fog.templ`, `.go`: `RoomFog(data RoomFogData)`, a `<form>`-per-
  floor list with `hx-trigger="room:tabletop from:window, room:fog from:window"`
  and `hx-sync="this:queue last"` on the root. Each floor: its name, a
  `toggle` named `enabled` posting on change, two radios named `prefill`
  labelled "Starts covered, shapes reveal" and "Starts clear, shapes hide",
  the shape count, Undo last (`hx-delete` to the newest shape's route,
  absent when there are none) and Clear (`hx-delete`, `hx-confirm` reading
  "Every fog shape on <name> goes. This cannot be undone.", in the error
  colour). `RoomFogData` carries a `RoomFogLayer` per floor with `ID`,
  `Name`, `Enabled`, `Prefill`, `Count` and `Newest`, and the paths each row
  posts to.
- `pages/room.go`: the Fog menu becomes one item, `Fog`, a `RoomWindow` with
  id `fog`, 320 by 360. `RoomTool` gains `Fogs` and `GM`. A
  `roomToolOptions()` template renders the second pill: `data-room-tool-options="fog"`,
  `hidden`, four buttons carrying `data-fog-shape="rect|poly"` and
  `data-fog-mode="reveal|hide"` with `aria-pressed`, an icon each and a
  tooltip; the polygon's tooltip is where "Enter closes it" is said.
- Words that must not appear in the new markup as attribute names, values or
  ids: `mask`, `filter`, `join`, `swap`, `list`. The mask is only ever a
  `mask` in TypeScript, which is not scanned. Run the selector diff after
  every change under `server/templ`.

## Client

### Modules

```
render/fog-pass.ts   the mask framebuffer, shape rasterisation, the cover and the tint
fog.ts               shape geometry, earcut, covered(), snapping, the gesture in hand
fog-tool.ts          the options pill: shape and mode, hidden while another tool is chosen
```

### The mask

`fog-pass.ts` keeps one `R8` texture in a framebuffer, the floor id, the
rectangle, the prefill and the list of shape ids it has drawn. `sync(shapes,
floorID, rect, prefill)` runs once per frame before the draw: when the floor,
the rectangle or the prefill differs, or the drawn ids are not a prefix of the
floor's shapes in order, it clears the mask to the prefill's value and draws
every shape; when the drawn ids are a proper prefix, it draws the new ones
onto what is there. Two thousand string comparisons per frame is nothing, and
frames do not happen while nothing changes, so there is no epoch to keep in
step. A reveal writes 1 and a hide writes 0 into the red channel; a covered
prefill clears to 0 and a clear prefill to 1.

The rasteriser is one program taking map-space triangles and the rectangle,
drawn with `TRIANGLES` from a buffer that grows by doubling. A rectangle's
two corners become two triangles; a polygon's ring goes through `earcut` and
its indices become one triangle per three.

The cover is the grid pass's shape with a different fragment shader: the
inverse clip matrix, the mask's rectangle, the mask sampler, the prefill and
the colour with its alpha. Inside the rectangle the shader samples the mask
with `LINEAR`, so a quarter-resolution edge is soft rather than stepped, and
outputs the colour where the sample is under a half; outside it, it outputs
the colour when the prefill is covered. `draw(role, ...)` is called at the
position decision 5 gives, and the renderer takes the colour from the same
probe `readClearColor` already runs.

### The tool

`fog.ts` exports `createFog(deps)` with `press(map, mods)`, `drag(map)`,
`release(map, mods)`, `hover(map)`, `key(e): boolean`, `abandon(): boolean`,
`outlines(out)`, `lines(out)` and `covered(x, y)`. `pawns.ts` asks
`deps.fogging()` after placement and before measuring in `press`, and hands
the gesture over for as long as the tool is chosen; `abandon()` asks the fog
first after the armed check, so Escape and the right button put a half-drawn
polygon away exactly as they put a ruler away. The Enter, Backspace and
Ctrl+Z keys go to `key()` from the document listener `pawns.ts` already has,
behind `typing()`, and only while the tool is chosen.

- Rectangle: press records the first corner, drag moves the second and the
  preview is one `RING_RECT` outline through `table.outlines` in a light tone
  for a reveal and a dark one for a hide; release with both corners at least a
  cell apart sends `fog.add` with the two snapped corners normalised to
  minimum then maximum, and a smaller one is a click and sends nothing.
- Polygon: each press adds a snapped corner; `lines(out)` gives the renderer
  the segments between them and a rubber band to the pointer, drawn through
  the over-marks path pass; Enter with three or more corners sends
  `fog.add` and starts a new polygon; Backspace takes the last corner back;
  Escape and the right button drop the lot.
- Both send the viewed floor, integers, and are preceded by `fog.setEnabled`
  when the floor's fog is off (decision 10). `covered()` reads the store and
  the viewed floor and is asked by `pawns.ts` through a `concealed(pawn)`
  dependency that is always false for the GM.

`fog-tool.ts` mounts the options pill: it reads the four buttons, keeps
`shape` and `mode`, sets `aria-pressed`, and hides or shows the pill from
`tools.onChange` by asking `tools.fogging()`. `tools.ts` grows `fogging()`
beside `panning()` and `measuring()`, asked of the CHOSEN button for the
reason `measuring()` is: the space bar borrows the pointer for the camera and
does not put the polygon in hand away.

### The renderer

`drawFrame` calls `fog.sync(...)` with the viewed floor's shapes, its
rectangle and its prefill after `layers.update`, and `fog.draw(...)` at the
role's position. The GM's viewed floor and a player's active floor both come
from `layers.viewed()`, which is already the floor everything else draws.
During a crossfade the mask is the incoming floor's from the first frame; a
fade of the fog itself is not worth a second mask.

`main.ts` changes by one line: `renderer.invalidate()` already runs on every
event, and the sync per frame is what notices the shapes changed. Player-side
hit testing, hovering and the marquee filter through `concealed`, so a pawn
under the cover is not found by any of them.

## Tests

**TypeScript**:

- `fog.test.ts`: a rectangle produces two triangles in map coordinates with
  corners normalised; a concave polygon triangulates with no degenerate
  triangle; `covered()` answers the prefill for a point in no shape, a reveal
  over a covered floor, a hide drawn over that reveal, and ignores a shape on
  another floor; corners snap to vertices under `cells` and `halfCells`, not
  under `off`, and not with Alt; a rectangle smaller than a cell is not sent.
- `pawns.test.ts` gains: a fog press on a floor with fog off sends
  `fog.setEnabled` and then `fog.add`; Escape mid-polygon sends nothing;
  Enter with two corners sends nothing and with three sends one `fog.add`;
  a player's hover, press and marquee skip a concealed pawn and not their
  own.
- `tools.test.ts` gains `fogging()`: true for the chosen Fog button, still
  true while the space bar is held.

**Go**:

- The three routes dispatch the right command for the layer or shape in the
  path; a player is refused on every one with a 403 alert.
- `SetLayerFog` dispatches one command when one flag changed, two when both
  did, and none when nothing did.
- `hub.Table` counts the shapes per floor and names the newest.
- Template tests pin the window's two-event trigger and `hx-sync`, that Undo
  last is absent on a floor with no shapes, that the Fog tool renders only
  for the GM, and that exactly one tool carries `data-room-tool-fogs`.

## Verification

1. `make js && make check` and the CSS selector diff; every added selector
   should be one the window or the options pill uses.
2. GM opens the Fog window, turns fog on for the ground floor with Covered
   chosen: the player's table is a solid block of the table colour and the
   GM's map is tinted. GM drags a rectangle reveal and clicks out a polygon
   reveal; the player sees the map there and nothing else, and no pawn
   outside the reveals. GM draws a hide polygon over part of a reveal: it
   closes back up on both screens.
3. Player hovers where a hidden goblin stands under the cover: no label.
   Player marquees across it: not selected. GM reveals it: the goblin appears
   and the label works.
4. Ctrl+Z in the GM's Fog mode removes the newest shape on both screens; Undo
   last in the window removes the next; Clear empties the floor behind the
   confirm.
5. Switch the floor to Clear in the window: the map shows everywhere and the
   shapes now mean the opposite. Turn fog off: the shapes stay and the
   window's count says so; turn it on again and they are back.
6. Fog the ground floor, then make the first floor active: the player sees an
   unfogged first floor. Switch back: the fog is exactly as left. View the
   first floor as the GM and draw a reveal there while the party is
   downstairs: the player's screen does not change.
7. Drag a pawn off the map's edge on a prefilled floor: the player does not
   see it beyond the edge either. Zoom out until the whole map is a stamp:
   the cover still ends where the image ends and the prefill continues past
   it.
8. Kill and restart the server mid-fight: both clients reconnect with the
   fog exactly as it was. Change the theme: the cover follows the table
   colour.
9. Benchmark with the mask on and two hundred shapes; record the numbers
   against phase 5.

## Out of scope

Server-side fog-aware projection (still deferred in the overview), a brush
or eraser tool for fog, per-shape editing after the fact, dynamic lighting
and line of sight, fog on a per-player basis, and strokes and pings, which
are phase 8.
