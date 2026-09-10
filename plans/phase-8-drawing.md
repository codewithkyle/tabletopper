# Phase 8: drawing

Read `plans/vtt-overview.md` (Drawing and erasing under Table features),
`plans/phase-2-protocol-core.md` for the stroke commands, and
`plans/phase-7-fog.md`, whose options pill, tool wiring, undo gesture and
pass placement this phase copies. This phase lets everybody at the table draw
on it: a pen, three measured shapes, an eraser, a colour and a width.

**Written 2026-09-10** against phase 7's shipped code, replacing the sketch
that was carried over when the old phase 6 was split on 2026-09-09. Pings were
in that sketch and are now `plans/phase-9-pings.md`: they share nothing with
drawing but a pill button, and folding them in would have put a two-second
fading ring in the way of the shape tools' verification.

Six things moved from the sketch and each is marked *Changed* below: strokes
gain a kind, shapes do not stream, the colour is a picker rather than eight
swatches, the width is a slider, both of those open beside the pill rather
than inside it, and the distance label is a persisted property of a shape
rather than a drag affordance.

## Already built

Server, and none of it needs changing except where decision 1 says so:

- `internal/room/stroke.go`: `Stroke{ID, By, LayerID, Color, Width, Points,
  Done}`. **The client mints the id**; the server checks only that it parses
  and is unused. `stroke.begin` is the GM's, or a player's when
  `Table.PlayersCanDraw`, on the floor a player is looking at; `stroke.extend`
  and `stroke.end` are the author's, the GM included; `stroke.erase` takes a
  list and is **the GM's or an author's own**, which is exactly the rule this
  phase wants and so is left alone; `stroke.clear` names a floor.
- `Table.PlayersCanDraw` defaults to **true** (`state.go`) and is already a
  toggle on Grid & settings reading "Players may draw on the tabletop".
- `StrokeWidthMax` 64, `StrokeChunkMax` 512, `StrokePointsMax` 20,000,
  `StrokesMax` 5,000, `StrokePointsBudget` 200,000, `CoordLimit` a million.
  `checkColor` accepts `#RRGGBB` and `#RRGGBBAA`; `checkPoints` bounds a flat
  pair array; `strokeBudget` bounds the room and the author.
- The reducer applies all five events on both sides, and `stroke.extended` is
  the third hot path. `main.ts` already consumes `stroke.cleared` to wipe a
  floor's blood, and `table.clear` sends one per floor.
- Snapshot migrations: `Schema` is 2, `migrations` is one step per past
  schema, and the golden snapshots in `testdata` pin every one of them.

Client, all of which this phase reuses rather than rewrites:

- The pill's Draw button exists with **no behaviour flag and no key**, so it
  currently leaves the table behaving as Select. `tools.ts` reads what a
  button does off `data-room-tool-pans`, `-measures`, `-fogs` and its letter
  off `data-room-tool-key`.
- `fog.ts` is the whole gesture contract this phase needs, already working:
  `press`/`drag`/`release`, `secondary()` for the right button, `abandon()`
  for Escape, `key(e)` for Ctrl+Z, `outline()` and `marks(out)` for the
  preview. `pawns.ts` routes all of them.
- `render/ring-pass.ts` draws hollow ellipses and rotatable rectangles at any
  radius, thickness in device pixels.
- `render/path-pass.ts` draws lines with a dark halo and `label(text, x, y,
  color, alpha)` from the glyph atlas. **Two instances exist**, one under the
  pawns and one over them.
- `render/glyphs.ts` rasterises exactly `0123456789 ft.` and nothing else. A
  character with no quad renders a hole.
- `path.ts`'s `feetBetween(dx, dy, grid)` is straight-line distance in the
  table's own units, and `distanceLabel(feet)` is the string.
- `actorColor(id)` in `pawns.ts`: eight colours from a hash of a player id,
  already used for other people's ghosts and rulers.
- `vanilla-colorful` is a dependency and `color.ts` is the established
  disclosure pattern -- a button, a chip, a folded picker, a live field.
- `fog-menu.ts` finds **every** `[data-room-layered]` on the page and keeps
  its `hx-vals` pointed at the viewed floor, and `RoomMenuItem.Layered`
  renders that attribute. A second menu item wanting the viewed floor is a
  field on a struct and no new JavaScript at all.

Not built: any stroke rendering, the draw tool, the options pill, the
`Kind` field, and the clear route.

## End state

- Anybody may choose **Draw** in the pill -- fifth button, key `d`, for every
  role -- and gets a second pill under the first with five modes (Pen,
  Rectangle, Circle, Cone, Erase), a round colour chip and a width control.
  The chip and the width open a panel **beside** the pill; only one is open at
  a time.
- The **pen** is freehand and streams as it is drawn, so the other people at
  the table watch the line form.
- The **rectangle** is dragged corner to corner. The **circle** is dragged
  from its centre. The **cone** is dragged from its point. All three preview
  live under the hand, land on the other screens when the button comes up, and
  are abandoned by Escape or the right button.
- The circle and the cone carry a **distance in feet**, and it stays on them:
  a 20 ft. circle sitting on the table is what the party is standing in for
  the next ten rounds. The rectangle carries one per edge. The pen carries
  none.
- **Erase** rubs out whole strokes under the pointer. A player's eraser takes
  out only their own lines; the GM's takes out anybody's.
- **Ctrl+Z** removes the viewer's newest finished stroke on the floor they are
  looking at, which is fog's gesture exactly.
- The GM's **Tabletop > Clear drawing** empties the floor they are looking at,
  behind the confirm modal.
- Players see drawing **under the fog**, so a note the GM leaves in an
  unrevealed room stays unrevealed.
- A reconnect, a snapshot and a restart all bring the drawing back exactly.

## Decisions

1. **A stroke gains a `Kind`, and a shape is geometry rather than a bag of
   points.** *Changed.* The sketch was freehand only. `Stroke.Kind` is
   `free | rect | circle | cone`, the way `FogShape.Kind` is `rect | poly`,
   and `Points` means something different in each.

   **The alternative was tessellating on the client and sending a polyline,
   which costs nothing on the wire and loses the shape.** A circle sent as
   sixty-four points is sixty-four points forever: it cannot be re-measured,
   so the label could only ever exist while the hand was still on the mouse.
   The whole reason the label was asked for is to size a spell, and a spell
   lasts longer than the drag -- so the geometry has to survive, and four
   integers is a cheaper thing to keep than a hundred and thirty.

2. **Every shape is exactly four integers, and they are always two points.**

   | Kind | `Points` | Means |
   | --- | --- | --- |
   | `free` | 2n, n ≥ 1 | the polyline, as it always was |
   | `rect` | 4 | two opposite corners, normalised to min then max |
   | `circle` | 4 | the centre, then a point on the rim |
   | `cone` | 4 | the apex, then the midpoint of the base |

   **The radius is a point on the rim rather than a scalar** so that every
   kind stays a flat pair array in map pixels: `checkPoints` and `checkCoord`
   go on working unchanged, `CoordLimit` bounds it, and there is no second
   unit anywhere in the protocol. The same reasoning makes the cone an apex
   and a base midpoint rather than a length and an angle.

3. **A shape is finished the moment it exists, so it never streams.**
   `stroke.extend` **appends** points -- that is the one delta in the whole
   protocol -- and a rubber-banded rectangle changes its *second* corner on
   every frame, which an append cannot express. The choice was a new transient
   preview event, the way `pawn.dragging` works, or letting a shape appear
   whole on the other screens when the button comes up.

   It appears whole. Nobody at a table notices that somebody else's circle
   arrived complete instead of growing, and a preview event would be a fourth
   hot path for a gesture that lasts a second.

   So `StrokeBegin.Apply` sets `Done: true` for any kind but `free`, and
   `StrokeExtend` refuses it with the message it already has for a finished
   stroke. **One command, no new field, and a rule that reads as what it is.**

4. **The cone is a cone and not a triangle: its base is as wide as it is
   long.** The gesture asked for was an isosceles triangle whose apex lands on
   the press and whose base follows the drag, and that is exactly what this
   is -- with the one free parameter, the base width, pinned to the length.

   That ratio is the 5e cone, which means the number under the shape is the
   number printed in the spell: drag until it says `30 ft.` and it is a
   thirty-foot cone. Left free, the GM would have two things to aim at and a
   label that answered neither question. If a free triangle is ever wanted it
   is a modifier on the drag, not a sixth mode.

   **The pill's button says Cone**, for the same reason: the word names what
   the tool is for. Rectangle and Circle say what they draw because there is
   nothing else they are for.

5. **A shape carries its distance and the pen carries none.** *Changed.* The
   sketch had no labels at all. What each says:

   - **Circle**: the radius. Spells are written "20-foot radius", so that is
     the number that can be read straight off.
   - **Cone**: the apex-to-base distance, which by decision 4 is the cone's
     own size.
   - **Rectangle**: one per edge, one horizontal and one vertical.
   - **Pen**: nothing. A freehand squiggle has no distance anybody asked for,
     and a label per line would bury the table in text.

   **The vocabulary is fourteen characters.** `glyphs.ts` holds
   `0123456789 ft.` and `distanceLabel` is written so it cannot produce
   anything outside it -- a character with no quad renders a hole. So a label
   is `30 ft.` and can never be `20 ft radius` or `30 x 15`. Nothing in this
   phase extends the atlas; if that is ever wanted it is a separate change
   with a separate reason.

   Labels are drawn through the **over** instance of the path pass, which is
   the same one the ruler's own label uses, so they read over the pawns
   standing in the shape.

6. **Everything tessellates into one stroke pass at buffer-build time.** The
   pass is the sketch's decision 1 and is unchanged: instanced segments, each
   carrying two endpoints, a width and a colour, with the fragment shader
   computing distance to the segment for a round-capped anti-aliased line. One
   draw for every finished stroke on the viewed floor from a buffer rebuilt on
   `erased`, `cleared` and a change of floor; one small draw for the strokes
   in progress, rebuilt per frame while any exists.

   A shape is expanded into segments **when that buffer is built**, not when
   it is stored and not per frame. A rectangle is four, a cone is three, and a
   circle is one every four map pixels, clamped to between 24 and 512.

   **The tessellation error is not a thing anybody can see.** A chord four map
   pixels long on a circle of radius *r* is off by about `2/r` map pixels at
   its middle, so a hundred-pixel circle is wrong by a fiftieth of a map pixel
   -- below a device pixel at any zoom this camera reaches.

   **The alternative was drawing circles through the ring pass**, which is
   already perfect at every zoom. It was refused because it would put circles
   in a different pass from every other stroke: a second buffer, a second
   z-position, and two rebuild paths for one feature. Perfection that cannot
   be perceived is not worth that.

7. **Width is in map pixels, with a device-pixel floor when it is drawn.** Ink
   is on the map: a line drawn across a corridor stays across that corridor at
   every zoom, which is what a map-space width means and what a screen-space
   one would not do. The floor exists because a two-pixel stroke on a map
   zoomed right out is a sub-pixel line that flickers in and out of existence;
   the shader clamps the rendered half-width to about three quarters of a
   device pixel, which is what the ring pass already does with thickness in
   the other direction.

8. **Erase removes whole strokes, and the authority is already right.** The
   eraser tests the pointer against segments on the CPU behind a bounding-box
   prefilter and sends **one** `stroke.erase` per pointer-up with every id it
   crossed. A pixel eraser would make a texture the truth and lose undo.

   `StrokeErase.Authorize` is already *GM erases anything, an author erases
   their own* and stays exactly as it is. A player's eraser therefore ignores
   other people's lines in the hit test as well, rather than sending ids the
   server will refuse: **the client filters, the server enforces, and neither
   trusts the other.**

9. **Undo is Ctrl+Z and it is fog's undo.** In any Draw mode it sends
   `stroke.erase` for the viewer's newest **finished** stroke on the floor
   being viewed, from the store's own copy. It is deliberately one level at a
   time with no redo and no undoing of an erase -- which is what
   `fog.ts:undo()` is, and consistency between the two tools is worth more
   here than a history nobody asked for.

10. **Draw is the pill's fifth button, key `d`, and it is everybody's.**
    `RoomTool` gains `Draws`; `RoomTools()` gives the existing Draw entry
    `Draws: true, Key: "d"` and **no** `GM: true`, so a player's pill keeps
    it. `tools.ts` grows `drawing()` beside `fogging()`, asked of the
    **chosen** button for the reason `measuring()` is: the space bar borrows
    the pointer for the camera and must not put a half-drawn cone away.

    A player in a room whose GM has turned drawing off is refused by the core
    with the alert modal. Hiding the button from them live would mean
    following `table.updated` to add and remove a pill button underneath a
    hand that is using it, and the refusal already says why.

11. **The options pill holds five modes, a colour chip and a width button, and
    the last two open panels beside it.** *Changed.* The sketch had eight
    fixed swatches and an unspecified width control, both inline.

    The pill itself stays **under** the main pill, which is phase 7's decision
    8 and its reasoning is untouched: the main pill is a vertical column at
    the top right and its tooltips open to the left, which is where a second
    pill beside it would sit.

    **The two panels open to the LEFT of the options pill**, anchored to the
    row of the button that opened them: `absolute right-full top-1/2
    -translate-y-1/2 mr-2` on a `relative` wrapper. Pure CSS, no measured
    geometry, and nothing in TypeScript that writes a position.

    They are panels rather than inline controls because 176 pixels of picker
    and a slider wide enough to aim at would double the pill's width and be on
    screen the whole time somebody is drawing. **One at a time**: opening one
    closes the other, choosing a mode closes both, Escape closes both, and a
    press on the canvas closes both.

12. **The colour is a full picker with no alpha, defaulting to the drawer's own
    `actorColor`.** *Changed.* `vanilla-colorful`'s `<hex-color-picker>` --
    the no-alpha element, beside the `<hex-alpha-color-picker>` `main.ts`
    already registers. Its every style is inside a shadow root, so there is
    nothing for Tailwind to scan and nothing to re-theme.

    **No alpha, deliberately.** `checkColor` accepts `#RRGGBB`, the stroke
    shader takes an opaque colour, and a half-transparent line over a map is a
    line nobody can see -- the grid needed alpha because it is a lattice laid
    over a picture, and a drawing is a mark on it.

    The default is `actorColor(user)`, so a player's lines match the ghosts
    and rulers the rest of the table already sees them draw. It is a starting
    value and not a rule: the picker overrides it and the choice is the
    viewer's for the session.

    **It is the tool's state and not the room's, and it is not persisted.**
    Nothing is stored and nothing is sent except inside `stroke.begin`, which
    is the same rule `fog-tool.ts` follows and for the same reason: a window
    would refetch and reset it, and `localStorage` would be a third place a
    colour lives.

13. **The width is a folded `range`, 1 to 64, defaulting to 4.** Sixty-four is
    `StrokeWidthMax` and the panel simply exposes the whole of it. Four map
    pixels is a pen line on a seventy-pixel cell.

    This is the one place in the room markup where the word `range` is
    legitimately a DaisyUI component: `class="range range-xs"` on the input.
    The selector diff will show `.range` gaining rules that are actually used,
    which is the first time that has been true -- it has been in the build all
    along off `for _, x := range xs` in a `.templ` file.

14. **Strokes go under the pawns, and under the fog for a player.** A drawing
    is on the floor: it goes after the blood and before the ruler's cells and
    the pawns, so a creature standing in a circle stands *in* it. The labels
    go over everything but the ruler, through the over-marks pass.

    For a player the cover is drawn after the ghosts and handles, so the
    stroke pass being under the pawns puts it under the cover as well and a
    note left in an unrevealed room stays unrevealed. Nothing about the fog
    changes.

15. **The client mints the ULID, and it is thirty lines rather than a
    dependency.** 48 bits of `Date.now()` and 80 bits from
    `crypto.getRandomValues`, Crockford base32, 26 characters -- which is what
    `github.com/oklog/ulid/v2` unmarshals from JSON. A new `ulid.ts` with a
    test that it is 26 characters, in the alphabet, and monotonic across a
    millisecond boundary.

    `path.ts` writes its own supercover rasteriser and `fog.ts` writes its own
    ear clipping for this reason: a dependency in the room bundle ships to
    every player at the table, and this is smaller than the import statement's
    share of it.

16. **The pen decimates on input and chunks at ten hertz.** With
    `getCoalescedEvents`, a point is kept only when it is at least one **map**
    pixel from the last kept point; chunks go out every 100 milliseconds or 64
    points, whichever comes first. A fast stylus and a slow mouse cost the
    same, which is `render/input.ts`'s rule applied to the socket instead of
    to the frame.

17. **The viewer's own stroke is drawn from local points until
    `stroke.ended`.** The echo of `began` and `extended` for a stroke this
    client is drawing is ignored; when the end arrives the store's copy
    replaces the local one and the finished buffer takes it. Otherwise the
    line under the hand would lag the hand by a round trip.

18. **The eraser has a radius and shows it.** Six CSS pixels converted to map
    pixels, plus half the stroke's own width, tested against each segment. The
    cursor is a hollow circle at that radius through the ring pass, which
    costs one instance and is the only thing that makes an invisible hit
    radius aimable.

19. **Nothing in this tool snaps to the grid.** Fog snaps because a reveal is
    a room and rooms are drawn on the lattice. A spell is placed against the
    creatures it is meant to catch, not against the floor, and the label
    already answers the question snapping would have answered. If it is ever
    wanted it is `snapCorner` and a modifier away.

20. **`Clear drawing` is a Tabletop menu item, the GM's, on the viewed
    floor.** *Changed.* The sketch put it on the options pill behind a hidden
    button. It belongs beside `Clear blood` and `Clear tabletop`, which are
    the other two verbs that empty the whole floor at once, and putting it
    there costs no new client code at all: `Layered: true` renders
    `data-room-layered` and `fog-menu.ts` already writes the viewed floor into
    every one of those it finds.

## Server

### State

`room.Stroke` gains one field:

```go
Kind StrokeKind `json:"kind"`
```

and `StrokeKind` is a string enum beside `ShapeKind`, with `Values()` and
`Valid()` in the pattern every other closed set in `state.go` uses:
`free`, `rect`, `circle`, `cone`.

`StrokeBegin` gains the same field. `Apply` validates it before anything else
about the shape:

- `!c.Kind.Valid()` is `invalid("Bad stroke", "That is not a kind of
  drawing.")`.
- `free` keeps today's rule: `checkPoints("stroke", c.Points, 1,
  StrokeChunkMax)`.
- Every other kind is **exactly four coordinates**: `checkPoints("stroke",
  c.Points, 2, 4)` plus an explicit `len(c.Points) != 4` refusal, so a
  three-point cone is a message rather than a shape drawn from whatever
  happens to be in the slice.
- A `circle` whose rim point equals its centre and a `cone` whose base
  midpoint equals its apex are `invalid("Bad stroke", "That shape has no
  size.")`. Both are what a click rather than a drag produces, and the client
  already drops them -- this is the server not trusting it.
- `Done` is set to `c.Kind != StrokeFree` when the stroke is appended.

Everything else in `stroke.go` is untouched: the budget, the id check, the
width and colour checks, `StrokeExtend`'s refusal of a finished stroke,
`StrokeErase`'s authority, `StrokeClear`.

### Snapshot

`Schema` goes to **3** and `migrations[2] = migrateStrokeKind`: every stroke
in a schema-2 snapshot is freehand, so the step sets `kind` to `"free"` on
each and touches nothing else. It is fifteen lines against
`migrateFootprints`'s eighty.

**A zero value would have done and is refused on purpose.** `""` is not a
member of the enum, so a room restored without the step would hold strokes
whose kind fails `Valid()` -- and the one thing a version number exists for is
to turn that into a loud failure rather than a stroke that renders as nothing.
`testdata` gains a schema-2 golden snapshot with strokes in it.

### Route

| Pattern | Wrapper | Handler | Behaviour |
| --- | --- | --- | --- |
| `POST /rooms/{id}/drawing/clear` | `RequireSession` | `ClearLayerDrawing` | GM only, behind `hx-confirm`. Reads `layer` from the form. Dispatches `stroke.clear`. 204. |

POST with the layer as a **form value** rather than `DELETE` with it in the
path, for phase 7's reason twice over: `roomBarItem` renders a button carrying
`hx-post`, and htmx captures a path when it processes an element so a layer
written afterwards is ignored. A `layer` that is not a ULID is a 404 with an
empty body; one the room does not hold is the core's refusal through
`rejectCommand`, which is the alert modal.

204 and no body: `stroke.cleared` is already what redraws both canvases, and
`main.ts` already wipes that floor's blood on it.

### Templates

- `pages/room.go`: `RoomTool` gains `Draws bool`, documented as
  `Measures`/`Fogs` are -- the mode whose primary button lays down ink. The
  Draw entry in `RoomTools()` becomes `{Name: RoomToolDraw, Label: "Draw",
  Draws: true, Key: "d"}` with a `RoomToolDraw = "draw"` constant beside the
  other three. No `GM: true`.
- `pages/room.go`: `DrawModeChoices()` beside `FogShapeChoices()`, five
  entries with a label and a hint each. The cone's hint is where "Drag from
  the point" is said and the circle's is "Drag from the centre", because
  those are the two gestures nothing else on this table teaches.
- `pages/room.go`: `tabletopMenu()`'s GM branch gains `Clear drawing` between
  `Clear blood` and `Clear tabletop`, carrying `Post: d.DrawingClearPath()`,
  `Layered: true`, a confirm and `ConfirmLabel: "Clear drawing"`. It is not
  `Danger` -- `Clear tabletop` is the one destructive item in that menu and
  sits last.
- `pages/room.templ`: `roomDrawOptions()` under `roomFogOptions()`, rendered
  for **every** role rather than inside `if data.IsGM()`. It is
  `data-draw-options`, `hidden`, and holds the five mode buttons
  (`data-draw-mode`, `aria-pressed`, an icon, a `tooltip-left` each, mirroring
  `roomFogGroup`), a divider, then the colour chip and the width button.
- The two disclosures are each a `relative` wrapper holding a button
  (`data-draw-open="color"`, `data-draw-open="width"`, `aria-expanded`,
  `aria-label`, and **no tooltip** -- a tooltip opening left under a panel
  opening left is two things in one place) and a `hidden` panel
  (`data-draw-popout="color"`, `data-draw-popout="width"`) positioned
  `absolute right-full top-1/2 -translate-y-1/2 mr-2`.
- The colour chip is a `btn btn-circle btn-ghost btn-sm` holding a round
  `data-draw-swatch` span over a white backing, which is `color.ts`'s honest
  backing for the same reason. Its first paint is the template's; every one
  after it is an inline `background-color`.
- The colour panel holds `<hex-color-picker data-draw-picker>` and nothing
  else. The width panel holds `<input type="range" data-draw-width min="1"
  max="64" value="4" class="range range-xs">` and a `data-draw-width-value`
  span.
- **Words that must not appear as an attribute name, value or id in the new
  markup**: `drawer`, `label`, `swap`, `stack`, `list`, `table`, `status`,
  `filter`, `join`, `mask`, `visible`, `indicator`, `divider`, `toggle`. The
  extractor splits on `:` and `.`, so this includes anything inside an
  `hx-trigger` or a compound value. Run the selector diff after every change
  under `server/templ`:

  ```sh
  cp server/public/css/app.css /tmp/app.before.css && make css
  diff <(grep -oE '^\s*\.[^ {,:]+' /tmp/app.before.css | sort -u) \
       <(grep -oE '^\s*\.[^ {,:]+' server/public/css/app.css | sort -u)
  ```

  The expected gains are `.range` and its friends, and nothing else.

## Client

### Modules

```
render/stroke-pass.ts   the instanced segment pass, the finished buffer, the live buffer
draw.ts                 the geometry, the gestures, the eraser, undo, the labels
draw-tool.ts            the options pill: mode, colour, width, the two panels
ulid.ts                 26 characters
```

`fog-menu.ts` is renamed **`layered-menu.ts`**. It never mentioned fog in its
body -- it finds `[data-room-layered]` and writes `hx-vals` -- and it now
serves two menus, so the name was already describing its first caller rather
than its job.

### The pass

`render/stroke-pass.ts` keeps two buffers and one program.

The **finished** buffer holds every done stroke on the viewed floor, expanded
to segments by decision 6, rebuilt when the floor changes or when the set of
done strokes on it changes. Its signature is the floor, the count, and the
newest id -- the same trick `fog-pass.ts` uses, and it catches an add, an
erase and a clear for the same reason: ids are minted in order and a new
stroke is appended.

The **live** buffer holds strokes that are not done -- the viewer's own from
local points per decision 17, and everybody else's from the store -- and is
rebuilt per frame while any exists. It is empty the rest of the time, which is
almost always, and an empty buffer is a skipped draw.

An instance is two endpoints, a width and a colour. The vertex shader expands
it to a quad grown by the half-width; the fragment shader computes the
distance from the pixel to the segment and produces a round cap and an
anti-aliased edge. Round caps mean adjacent segments need no join at all,
which is what makes a polyline one buffer rather than a mesh.

`draw(...)` is called at decision 14's position, and `labels` are pushed
through `Table.marks`' sibling rather than through this pass.

### The tool

`draw.ts` exports `createDraw(deps)` with the shape `fog.ts` established:
`press(map, mods)`, `drag(map, mods)`, `release(map, mods)`, `secondary()`,
`hover(map)`, `key(e): boolean`, `abandon(): boolean`, `active(): boolean`,
`outline()`, `marks(out)` and a new `labels(out)`.

`deps` mirrors `FogDeps`: `state`, `role`, `user`, `viewed`, `grid`, `send`,
`invalidate`, `drawing: () => boolean` and `options: () => DrawOptions`. The
options are read **when a gesture finishes** rather than when it began, which
is `fog.ts`'s rule and lets somebody change their mind about a colour with the
button still down.

Per mode:

- **Pen**: press mints a ULID and sends `stroke.begin` with the first point,
  the kind `free`, the colour and the width; drag decimates and chunks per
  decision 16; release flushes what is left and sends `stroke.end`. Escape and
  the right button mid-stroke send `stroke.end` and then `stroke.erase` for
  it -- **a stroke that has begun cannot be un-begun**, because the other
  screens have already seen it, so abandoning is drawing and rubbing out.
- **Rectangle**: press records a corner, drag moves the other, the preview is
  one reused `RING_RECT` `Outline` through `outline()`. Release with both axes
  differing sends `stroke.begin` with the two corners normalised, then
  `stroke.end`.
- **Circle**: press records the centre, drag moves the rim, the preview is one
  `RING_ELLIPSE` `Outline` with equal half-axes. Release with a non-zero
  radius sends the centre and the rim point.
- **Cone**: press records the apex, drag moves the base midpoint, the preview
  is the three segments through `marks()`. Release with a non-zero length
  sends the apex and the base midpoint. The base's half-width is the length
  halved, computed here and never sent.
- **Erase**: press starts a set, drag adds every stroke id whose expanded
  segments come within decision 18's radius **and** which this viewer may
  erase, release sends one `stroke.erase` with the set. The cursor ring is one
  `Outline` while the mode is chosen and the pointer is over the table.

`labels(out)` pushes the in-hand shape's label **and** every finished shape's
on the viewed floor, so one accessor answers both and the renderer stays dumb
about which is which. `Label` is `{text, x, y, color, alpha}`, drawn through
the over-marks path pass beside `marks`.

`pawns.ts` gains the draw tool where it gained the fog one: `deps.drawing()`
is asked in `press` after placement and before measuring; `secondary()` routes
to the draw tool before `abandon()`; the document key listener passes Ctrl+Z
through `key()` behind `typing()`. `Table` gains `labels(out)` beside
`marks(out)`, and `renderer.ts` drains it into the over path pass.

`effects.ts`'s `touchesPawns` gains **nothing** -- concealment does not depend
on strokes -- but `renderer.invalidate()` already runs on every event, which is
what makes a stroke arriving from somebody else draw.

### The pill

`draw-tool.ts` is `fog-tool.ts` with three more controls, and it writes no
class name for the reason that file writes none: `server/js` is not a Tailwind
source. It sets `aria-pressed`, `aria-expanded`, `[hidden]`, one inline
`background-color` and one text node.

- Mode: five buttons, one `aria-pressed`, the value taken as written the way
  `fog-tool.ts` takes the fog's -- the closed set is validated in Go.
- Colour: the chip toggles the colour panel and closes the width one;
  `color-changed` from the picker writes the chip's inline background and the
  tool's colour. **It is not `color.ts`.** That module is bound to a form
  field, settles a `change` for htmx and speaks `#RRGGBBAA`; none of those
  are true here, and the twenty lines this needs are less than the branches
  teaching that module about a control with no form behind it.
- Width: the button toggles the width panel and closes the colour one; `input`
  on the range writes the number into the tool and the text into the span.
- One panel at a time, and both close on a mode click, on Escape, and on a
  press on the canvas.
- The pill follows `tools.onChange` by asking `tools.drawing()`, and closes
  both panels when the tool goes away.

`main.ts` imports `vanilla-colorful/hex-color-picker.js` beside the alpha one,
at module scope, for the reason already written there: it runs
`customElements.define` and the tests beside these files run in node.

## Checkpoints

Five, each one a thing that can be run and played with. `make run` after each;
`make check` and the selector diff are part of every one and are not repeated
below.

### Checkpoint 1 -- the pen draws

**Server**: `StrokeKind`, the `Kind` field, the validation, `Done` on begin,
`Schema` 3 and `migrateStrokeKind`, the golden snapshot. **The whole enum
lands here even though only `free` is used**, so there is one schema change
and one migration for the phase rather than two.

**Client**: `ulid.ts`, `render/stroke-pass.ts`, `draw.ts` with the pen alone,
`RoomTool.Draws` and the key `d`, `tools.ts:drawing()`, the wiring in
`pawns.ts` and `renderer.ts`. Colour is `actorColor(user)` and width is a
constant 4; there is no options pill yet.

**Verify**: two browsers on one room, a GM and a player. Draw a squiggle in
each: it appears on the other as it is drawn, in that person's own colour.
Reload both -- both squiggles come back. Restart the server mid-session --
they come back again, which is the migration. Turn `Players may draw` off in
Grid & settings: the player's next stroke raises the alert modal and the GM's
still works. Zoom right in and right out: the line stays on the map and stays
visible.

### Checkpoint 2 -- erase, undo and clear

**Server**: the `POST /rooms/{id}/drawing/clear` route and its handler.
Nothing in `stroke.go`.

**Client**: erase mode in `draw.ts` with the hit test, the radius and the
cursor ring; Ctrl+Z; `Clear drawing` in the Tabletop menu; `fog-menu.ts`
renamed to `layered-menu.ts`. Erase is reachable by a temporary key or a
second pill button until checkpoint 3 lands the real one.

**Verify**: the player rubs out their own line and it goes on both screens.
The player drags the eraser across the GM's line: nothing happens, and no
alert -- the client never sent it. The GM rubs out the player's line and it
goes. Ctrl+Z removes each person's own newest line, repeatedly, and stops when
there are none left. `Clear drawing` on the floor the GM is **looking at**
empties that floor behind the confirm and leaves the other floors alone; the
blood on that floor goes with it.

### Checkpoint 3 -- the options pill

**Server**: `DrawModeChoices()`, `roomDrawOptions()`, the two panels.

**Client**: `draw-tool.ts`, the `hex-color-picker` import, the options plumbed
into `draw.ts`.

**Verify**: choosing Draw shows the pill and choosing anything else hides it.
The colour chip opens a picker **beside** the pill; picking green and drawing
gives a green line on both screens. The width button opens a slider beside the
pill and closes the picker; at 1 the line is a hairline and at 64 it is most
of a cell. Opening one panel closes the other; Escape, a mode click and a
press on the canvas all close both. Switch tools and back: the colour and
width are still what they were. Reload: they are the defaults again, which is
decision 12.

### Checkpoint 4 -- rectangle and circle

**Client**: the two kinds in `draw.ts`, their previews through `outline()`,
their labels through `labels()`, and the shape expansion in the pass.

**Verify**: drag a rectangle -- it previews under the hand and lands on the
other screen whole when the button comes up, with a label on each edge. Drag a
circle from its centre -- the preview grows around the press point and the
label reads the radius. Set the grid to 5 feet per cell and check the numbers
against the cells. Escape mid-drag and right-click mid-drag both leave
nothing. Ctrl+Z removes a shape. Zoom right in on a circle: it is round.
Reload: both shapes come back with their labels. A player draws a circle over
the GM's fog: the GM sees it and the player does not see what is under it.

### Checkpoint 5 -- the cone

**Client**: the `cone` kind, its three-segment preview, its label.

**Verify**: press at the caster and drag -- the point stays put, the base
opens out with the drag, and the base is as wide as the shape is long. Drag
until the label reads `30 ft.` on a 5-foot grid: the base spans six cells.
Turn it through a full circle: the shape follows the pointer at every angle
and the label does not change with direction. Escape and right-click cancel.
Ctrl+Z removes it. Leave one on the table for a few minutes with pawns walking
through it: it stays, the label stays readable over the pawns, and nothing
about it costs a frame.

## Tests

**TypeScript**

- `ulid.test.ts`: 26 characters, Crockford alphabet only, and two minted in
  the same millisecond differ.
- `draw.test.ts`: the four kinds expand to the expected segments; a circle's
  segment count is bounded at both ends; the cone's base is as wide as its
  length and perpendicular to its axis; `feetBetween` drives each label and
  every label is inside `GLYPHS`; a click rather than a drag sends nothing for
  each of the three shapes; the eraser's hit test finds a stroke within the
  radius, misses one outside it, and a player's never returns somebody else's
  id.
- `pawns.test.ts` gains: a pen stroke sends one `begin`, at least one
  `extend`, and one `end`; Escape mid-pen sends `end` then `erase`; each shape
  sends exactly one `begin` and one `end` and no `extend`; Escape and the
  right button mid-shape send nothing; Ctrl+Z sends `erase` for the viewer's
  own newest done stroke on the viewed floor and nothing when there is none.
- `store.test.ts`: a `stroke.began` carrying a kind round-trips it.

**Go**

- `StrokeBegin` accepts each kind; refuses an unknown one; refuses a shape
  that is not four coordinates; refuses a zero-size circle and cone; sets
  `Done` for the three shapes and leaves it false for `free`.
- `StrokeExtend` on a shape is refused with the finished-stroke message.
- The clear route dispatches `stroke.clear` for the layer in the form; a
  player is refused with a 403 alert; a `layer` that is not a ULID is a 404.
- `migrateStrokeKind` turns a schema-2 snapshot's strokes into `free` ones and
  the golden-snapshot test covers schema 2 with strokes in it.
- Template tests: exactly one tool carries `data-room-tool-draws`; the Draw
  tool renders for a player as well as a GM; the options pill renders five
  `data-draw-mode` buttons for both roles; `Clear drawing` carries
  `hx-confirm` and `data-room-layered` and renders only for the GM.

## Out of scope

Pixel erasing, filled shapes, a polygon or free-line tool, text on the table,
snapping, layers of drawing within a floor, editing a shape after it has
landed, redo, per-viewer drawing that others cannot see, a stroke that follows
a pawn, and pings -- which are `plans/phase-9-pings.md`.
