# Phase 5: pawns

Read `plans/vtt-overview.md`, then `plans/phase-2-protocol-core.md` for the
pawn commands, the object kind and per-axis snapping, `plans/phase-3-transport.md`
for `Dispatch` and resolution, and `plans/phase-4-renderer.md` for the render
passes this adds to. This phase puts creatures and objects on the table:
spawning from the monster manual, the token library and the party's
characters; selecting one or many; moving them with snapping and the shared
movement path; and editing them through a dialog.

## As built

### Checkpoint 1, the server (2026-09-08)

Resolution, the write-through, the routes, the fragments and the window
plumbing shipped. Nine things differ from what the sections below sketch, each
for a reason the code carries in a comment:

- **Snapping modes are `off | cells | halfCells`, not "corners".** `snap.go`
  answers the parity question with cells mode and drops it in half-cells mode;
  there is no corners mode to port. `path.ts` is a port of `SnapAxis` as it is.
- **`pawn.spawn` gained a `size` field.** The wire carried no size at all, so a
  token could only ever land medium -- and size is the one stat that is
  structural rather than informational: it decides the footprint, which decides
  the snapping lattice and where a click actually puts the pawn. Hit points and
  armour class stay off the wire and are the pawn dialog's, per decision 10.
  Resolution reads the field; `Apply` is unchanged.
- **The room stat block needs no new statement.** `GetMonster` and
  `ListMonsterActions` both already take an `OwnerID` parameter, so "scoped to
  the room owner" is which id goes in rather than a second copy of the SQL.
  Decision 13's `GetMonsterStatBlockForRoom` would have been a byte-identical
  duplicate; the actions query, which decision 13 does not mention, would have
  needed one too and has the same answer.
- **The stat block is GM-only.** The route table said "any member when the pawn
  is visible", which contradicts the room's own monster-health setting: a stat
  block carries hit points, armour class and legendary actions, so serving one
  to a player undoes in one window what the GM chose in another.
- **The spawn dialog is two routes**, `/fragment/room/spawn` and
  `/fragment/room/spawn-list`, which is the shape the map picker and the asset
  manager already have. One route means a search swaps the box being typed into
  and the caret jumps to the end on every keystroke.
- **The conditions editor adds rows through a fragment**,
  `/fragment/room/condition-row`. `repeater.js` only REMOVES rows now; the
  character sheet adds them through `/fragment/character/feature-row`, and a row
  built in JavaScript would render unstyled because `public/js` is not a
  Tailwind source.
- **The write-through fires on every `pawn.updated` for a player pawn with a
  character**, rather than diffing hit points. Knowing that hit points changed
  means carrying the before-image through a path whose whole value is that an
  event carries the entity and nothing else; what it saves instead is a
  statement that writes the value already there, a few times a session.
- **`resolveParty` returns early for a player.** Resolution runs before
  Authorize -- the price of running it off the room's goroutine -- so a player
  pressing Spawn party was told "everybody already has a pawn", which is true,
  useless, and not why they were refused. Leaving `Pawns` nil hands the refusal
  to Authorize.
- **`hx-vals` on the DELETE arrives in the QUERY STRING.** This htmx tests
  `/GET|DELETE/` against the method and appends to the URL for both, while
  net/http's `ParseForm` reads a body only for POST, PUT and PATCH. `ParseForm`
  is exactly the union of the two; `r.PostForm` would have found nothing.

**The CSS diff caught one thing and it was the attribute vector.**
`list="condition-names"` -- HTML's own attribute, tying the condition field to
its `<datalist>` -- put DaisyUI's whole `.list` family in the build: fourteen
selectors and 2.4 KB. The word cannot be renamed, so the attribute is built in
`room-pawn.go` as a `templ.Attributes` and spread into the markup, which is the
same move the ban on prose makes for the same reason. Every other selector the
phase added is one the new markup uses.

**The menu items are not wired yet.** `Spawn pawns` and the player's
`Place my pawn` land with `dialogs.ts` in checkpoint 3, because an enabled menu
item whose click does nothing reads as broken -- which is the argument
`room.go` already makes for the disabled ones.

### Checkpoint 2, the rendering (2026-09-08)

The sprite cache, the three passes, the ruler's arithmetic, the layer filter and
the stress toggle shipped. Six things differ from what the sections below
sketch:

- **A sprite keeps its aspect ratio.** Decision 1 says "resized to 256 by 256",
  which is right for a monster picture and a portrait -- both are stored square
  already -- and wrong for the case the object kind exists for. A wagon is
  stored 512 by 171; squashed into a square and stretched across a two-by-four
  footprint it is visibly the wrong wagon. So the longest edge becomes 256, the
  layer is partly unwritten exactly as an edge tile's is, and the real pixels
  ride in the slot -- machinery the tile cache already had. Creatures then COVER
  their disc (cropped, because a portrait with bars down the side inside a
  circle looks like a mistake) and objects CONTAIN theirs.
- **`createImageBitmap` cannot fit, so the decode is two steps.** resizeWidth
  and resizeHeight are absolute: giving both distorts and giving one needs to
  know which edge is longer, which is what the first decode answers. The second
  call resizes an already-decoded bitmap, which is cheap and still off-thread.
- **The glyph atlas is a module of its own**, `render/glyphs.ts`. The plan asks
  for one without naming a file; it is fourteen characters rasterised once, and
  a label is an index into them rather than a canvas re-uploaded every time the
  dragged cell changes.
- **Hairlines are DEVICE pixels and text is CSS pixels.** A pawn border, a
  condition ring and its spacing are measured in the pixels that actually exist
  -- which is the argument the grid's own shader makes for drawing itself
  exactly one device pixel wide -- and the distance label is measured in the
  ones a person reads. Mixing them made rings twice as far apart as they were
  thick on a retina screen, which is how the two units got separated.
- **The sprite cache re-touches its own working set every frame.** The pawn
  buffer is rebuilt on a CHANGE and frames happen continuously, so a picture
  arriving several frames after the rebuild that asked for it would claim a
  layer while nothing had touched the ones on screen -- and with a full cache,
  claiming means evicting them. `begin(rebuilding)` is the fix: forget the set
  only when the caller is about to name a new one.
- **`render/scene.ts` holds the layer filter**, so rendering, hit testing, the
  marquee and the riders lookup all read one list. Phase 5's client module list
  has the filter as a property of four modules; one function is one place for it
  to be wrong.

**The shaders are checked with glslangValidator, not by looking at them.** All
five programs compile as GLSL ES 3.00 and LINK -- the second catches a varying
declared `flat` on one side and not the other, which is a link error and
produces a black table with no other symptom. The extractor lives in the
session scratchpad rather than the repo, because wiring it into `make check`
would make the build depend on a tool that is not currently required. It found
one real thing on the first run: `.replace("BORDER", ...)` on a shader source is
a runtime string operation nothing verifies, and a replace that matched nothing
would ship an uncompilable shader. It is a `#define` now.

**The CSS diff is empty.** The one markup change is the debug panel's Stress
button, which is `btn btn-xs` and already in the build.

**Nothing drives the path pass yet.** It is built, wired and drawn in the right
order -- under the pawns for the highlighted cells, over them for the line and
the label -- and the drag that fills it is checkpoint 3.

## End state

- The GM opens a Spawn dialog, searches monsters or tokens, picks one, and
  clicks the map to place it, again and again until Escape. A token can be
  placed as a creature or as an object with a rectangular footprint, such as a
  two by four wagon. A Visible toggle decides whether players see it appear.
  Spawn party places a pawn for every connected player who joined with a
  character.
- A player who joined with a character can place their own pawn.
- Anyone selects pawns they may move: click one, Shift-click to add, or drag a
  marquee over several. Dragging a selected pawn drags the whole selection as
  one. Dragging a wagon carries the pawns standing on it without selecting
  them first.
- With snapping on, the grabbed pawn snaps to cells or corners by footprint
  parity, the rest keep their exact offsets, a path of highlighted cells runs
  from where the grabbed pawn stood to where it will land, and a label shows
  the distance in feet under the table's diagonal rule. Every other client
  sees the same ghosts and path in the dragging player's colour. Release
  commits; Escape cancels.
- Hovering or selecting shows an overlay: for one pawn, its name, hit points
  as the viewer may see them, armour class and conditions, with buttons for
  the edit dialog and the stat block; for several, the count and a Remove
  button for the GM.
- The pawn dialog edits name, HP with arithmetic input, max HP, AC, size or
  footprint, layer, visibility, and conditions with colour and duration.
- Every pawn lives on one layer. The canvas shows the viewed layer's pawns;
  spawns land on the layer the spawner is viewing; the GM moves pawns between
  layers from the pawn dialog's layer select or the selection overlay's Move
  to layer control. When the GM switches the active layer, players see the
  floor change and its pawns arrive.
- A player pawn's hit points write through to the character sheet's current HP.
- The debug panel's stress button spawns 500 synthetic pawns client-side and
  the benchmark still reports frame times.

## Decisions

1. **Pawn images live in a second texture array** of 256-square RGBA8 layers,
   separate from tiles, LRU like them. Images are fetched through the same
   loader with `createImageBitmap` resized to 256 by 256 with
   `resizeQuality: "high"`. Sources: `/assets/images/{id}` for monster,
   character and token assets, or the session avatar's absolute URL for a
   player without a portrait. A creature with no image draws a disc in its
   kind's colour with its initials, rendered once into a layer from a 2D
   canvas.
2. **Creatures are discs, objects are rectangles.** A creature sprite is masked
   to a disc with a two-pixel border in the kind colour and its conditions draw
   as concentric rings outside the border, one per condition in its colour, at
   most sixteen. An object draws its image unmasked, fitted by aspect inside
   its footprint rectangle, with no border and no rings.
3. **Hit testing is on the CPU** over pawns in reverse draw order: a distance
   test for discs, a rectangle test for objects. A few hundred pawns is a few
   microseconds.
4. **Selection is local and explicit.** A set of pawn ids the viewer may move.
   Click selects one, Shift-click toggles one, Shift-drag on empty space or the
   Select tool draws a marquee in map space and selects every movable pawn
   whose centre falls inside. Click on empty space clears it. Nothing about
   selection is on the wire.
5. **A drag moves the selection with one delta.** The grabbed pawn is the
   anchor. Its ghost snaps; every other selected pawn's ghost is the anchor's
   delta applied to its committed position. The path and distance label belong
   to the anchor. Release sends `pawn.move` with the anchor, its snapped
   position and the other ids; the server computes the same delta and applies
   it, so the result is exactly what the ghosts showed.
6. **Riders are carried automatically.** When a drag starts on a pawn whose
   footprint is at least two cells on either axis, every pawn with a higher
   layer whose centre lies inside that footprint joins the drag, whether or
   not it was selected. Alt-drag moves the anchor alone. A wagon spawned before
   the party sits below them in layer order, so the rule needs no attach
   relationship, and it never triggers on a one-cell creature.
7. **Drag preview and path are computed locally by every client.** The dragging
   client sends `pawn.drag` when the snapped anchor cell changes, or at most
   twenty times a second when snapping is off. Other clients receive
   `pawn.dragging` with every previewed position and draw ghosts and the
   anchor's path. A `pawn.moved`, `pawn.updated` or `pawn.removed` touching the
   anchor ends the preview, as does three seconds without an update.
8. **A cancelled drag sends `pawn.move` back to the committed position** with
   the same others. The server emits `pawn.moved` with unchanged positions,
   which is what tells everyone else to drop the preview. No new event type.
9. **The overlay is one DOM element**, positioned by a single `transform` write
   per frame while a selection or hover exists and none otherwise. Its text is
   set when the selection changes, not per frame. Every class is in the
   templ; the script sets `textContent`, `hidden`, the transform and `hx-vals`.
10. **Editing a pawn is two surfaces and both post over HTTP.** The live panel
    in a window carries one editable control, HP, because that is the value
    that changes every round and a dialog in front of "the goblin takes 7" is
    the wrong shape; it posts to a route of its own and is answered with the
    panel, which is the mutation-returns-what-it-changed case the fragment
    rules name. Everything else -- name, size or footprint, max HP, AC,
    conditions, visibility, layer -- is a form in the content modal, which
    cannot be clobbered by a refetch because nothing refetches it. Both
    handlers build `pawn.update`, `pawn.setConditions`, `pawn.setVisible` and
    `pawn.setLayer` as needed and dispatch them. The HP field accepts `12`,
    `-7` or `+3`, evaluated server-side: an absolute value replaces, a signed
    value adds to the current. Errors from the core come back as 422 form
    errors.
11. **Removal goes through HTTP behind the confirm modal**, one pawn from the
    dialog or the whole selection from the overlay. There is no Delete key,
    because a keypress cannot pass through the confirm modal and a destructive
    action without one breaks the app's rule.
12. **Player pawn hit points write through to the sheet's `current_hp` only.**
    A new narrow statement `UpdateCharacterCurrentHP`, per the rule that a
    writer's columns are exactly the set it owns. Max HP and AC on a player
    pawn are read from the sheet at spawn and edited on the table without
    writing back.
13. **The room-scoped stat block route reads through the pawn.** `GetMonster`
    is owner-scoped and stays that way; the new fragment looks up the pawn in
    the live state, takes its `monsterId`, and reads the monster with a new
    query scoped to the room owner rather than the requester.
15. **Pawns are filtered by the viewed layer everywhere on the client.**
    Rendering, hit testing, marquee, riders and hover all read only pawns whose
    `layerId` is the viewed layer. `pawn.spawn` carries the viewed layer; a
    player's is always the active one, and the server refuses anything else.
    Moving pawns between layers is GM only through `pawn.setLayer`, because a
    player who sent their own pawn upstairs would no longer receive it.
14. **Distance is counted in cells along the path, not measured.** `equal`:
    the Chebyshev distance. `alternating`: diagonals cost one cell on odd
    counts and two on even, the 5-10-5 rule. Multiply by feet per cell. The
    path is the supercover line between the anchor's two snapped centres.

## Server

### Resolution (`hub/resolve.go`)

Replaces the phase 3 stubs for `pawn.spawn` and `pawn.spawnCharacters`. Runs
on the connection's goroutine with a two-second context.

| Wire | Lookup | Resolved pawn |
| --- | --- | --- |
| `kind: monster`, `monsterId` | `GetMonsterForRoom(monsterId, ownerId)`, owner is the room's GM | name, size normalised to the six (unknown becomes medium), `hp` and `maxHp` from `hp`, `ac`, image from `asset_id` when set, `monsterId` |
| `kind: npc`, `name`, optional `assetId` | `GetLibraryAsset(assetId, GM, token)` when given | name, size from the wire, `hp`, `maxHp`, `ac` from the wire (defaults 1, 1, 10), image, `ownerId` = the actor when the actor is a player |
| `kind: object`, `assetId`, `footprintW`, `footprintH`, optional `name` | `GetLibraryAsset(assetId, GM, token)` | name (the asset's name when blank), footprint from the wire within `FootprintMax`, image, `hp`, `maxHp`, `ac` nil, no conditions |
| `kind: player`, `characterId` | `GetCharacter(characterId, actor)` for a player; for the GM, `GetCharacterForRoom` scoped to any member's character | name, size normalised, `hp` = `current_hp`, `maxHp` = `max_hp`, `ac`, image from the portrait or the player's avatar, `characterId`, `ownerId` = the character's owner |
| `pawn.spawnCharacters` | for each connected player with a `characterId` and no existing player pawn with that `characterId` | as above, on the active layer, positions in a row at that layer's map centre spaced one cell apart |

Every spawn carries `layer` from the wire, and the hub passes it through
untouched; the core validates it.

New queries:

| File | Name | Kind | Statement |
| --- | --- | --- | --- |
| `monsters.sql` | `GetMonsterForRoom` | `:one` | `SELECT id, name, size, ac, hp, asset_id FROM monsters WHERE id = ? AND owner_id = ?` |
| `monsters.sql` | `GetMonsterStatBlockForRoom` | `:one` | the same columns `GetMonster` selects, `WHERE id = ? AND owner_id = ?`, used by the stat block fragment with the room owner as the owner |
| `characters.sql` | `GetCharacterForRoom` | `:one` | `SELECT id, owner_id, name, size, ac, current_hp, max_hp, asset_id FROM characters WHERE id = ?`; the handler checks the owner is a member |
| `characters.sql` | `UpdateCharacterCurrentHP` | `:execresult` | `UPDATE characters SET current_hp = ? WHERE id = ?` |

### Write-through

A hub side effect on `pawn.updated` where the pawn's `kind` is player and its
`hp` changed: run `UpdateCharacterCurrentHP` with the pawn's `characterId`.
Failures log; the table is still right, and the sheet catches up on the next
edit.

### Routes

| Pattern | Wrapper | Handler | Behaviour |
| --- | --- | --- | --- |
| `GET /fragment/room/spawn?room={id}&kind={monsters\|tokens}&q=` | `Fragment` | `RoomSpawnFragment` | GM only. Search box in the shape of `assetSearchBox`, results as pick cards. `kind` is matched against the two values before any statement. The tokens view carries a Creature or Object choice; Object reveals width and height fields in cells, defaulting to 1 and 1. A card arms placement with `hx-on:click` dispatching a `room:arm` window event carrying `{kind, id, name, visible, footprintW, footprintH}` read from the dialog's controls, then `modal:close`. A Spawn party button posts to the party route. |
| `POST /rooms/{id}/pawns/party` | `RequireSession` | `SpawnParty` | GM only. Dispatches `pawn.spawnCharacters`. `htmx.CloseModal`. |
| `GET /fragment/room/pawn?room={id}&pawn={id}` | `Fragment` | `RoomPawnFragment` | Any member; the projection decides what they see. Reads the live pawn via `hub.Pawn(ctx, roomID, pawnID, role)`, PROJECTED FOR THE ROLE; 404 empty if absent or not shown to the requester. Renders the live panel that goes in a window: name, HP, AC, conditions, layer, with HP editable inline. Objects show a footprint instead of a size and no conditions. See the last section of this document for the trigger it carries and why. |
| `GET /fragment/room/pawn/edit?room={id}&pawn={id}` | `Fragment` | `RoomPawnEditFragment` | Owner or GM of the pawn. The form that goes in the content modal: name, size or footprint, max HP, AC, conditions, and for the GM visibility and a Layer select listing the room's layers from the live table. |
| `POST /rooms/{id}/pawns/{pawn}` | `RequireSession` | `UpdatePawn` | The content modal's form. Parses it, dispatches `pawn.update`, then `pawn.setConditions` when the repeater changed, then `pawn.setVisible` when the GM toggled it, then `pawn.setLayer` when the GM changed the layer. `htmx.CloseModal`. |
| `POST /rooms/{id}/pawns/{pawn}/hp` | `RequireSession` | `UpdatePawnHP` | The window panel's one inline control. Evaluates the arithmetic against the live value, dispatches `pawn.update`, and answers with the panel fragment -- the mutation returning what it changed, so the person who typed it sees the result without waiting for their own event to come back round. NOT `htmx.CloseModal`: there is no modal open, and sending it would dismiss whatever else was. |
| `POST /rooms/{id}/pawns/layer` | `RequireSession` | `MovePawnsToLayer` | GM only. Reads repeated `ids` and `layer`, at most `SelectionMax`, dispatches `pawn.setLayer`. Empty 200. Used by the pawn dialog with one id and by the overlay with the selection. |
| `DELETE /rooms/{id}/pawns` | `RequireSession` | `RemovePawns` | GM only. Reads repeated `ids` form values, at most `SelectionMax`, dispatches one `pawn.remove`. Empty 200. Used by the dialog with one id and by the overlay with the selection. |
| `GET /fragment/room/stat-block?room={id}&pawn={id}` | `Fragment` | `RoomStatBlockFragment` | GM, or any member when the pawn is visible. Resolves the pawn's `monsterId`, reads with the room owner as owner, renders the same stat block component the manual uses. |

The Spawn dialog opens in the content modal at size `lg`. The pawn panel and
the stat block are **windows**; see "Windows, and how live data reaches them"
at the end of this document, which supersedes decision 10's "dialog" and the
last two rows of the table above. Buttons are solid `btn`; Close first.

### Templates

- `pages/room-spawn.templ`, `.go`: `RoomSpawnFragment(data RoomSpawnData)` and
  the two card partials. The monster card shows image, name, CR and size; the
  token card shows the image and name. The Creature or Object choice and the
  footprint fields sit above the token results.
- `pages/room-pawn.templ`, `.go`: `RoomPawnFragment(data RoomPawnData)`. The
  conditions editor uses `repeater.js` rows: name (with a `<datalist>` of the
  twenty familiar conditions from the old client), colour select of the eight
  colours, duration number with -1 meaning until removed, clear-on select.
  Visibility and layer controls render only for `RoleGM`. Remove is a button
  with `hx-delete` to the pawns route, `hx-vals` with the id, and `hx-confirm`.
- The room page gains the overlay element, hidden by default, with two shapes
  inside it toggled by `hidden`: the single-pawn block and the selection block.
  The selection block's Remove button carries `hx-delete`, `hx-confirm` with a
  heading override, and an `hx-vals` the client fills with the selected ids.
  For the GM the selection block also carries a Move to layer `<select>` whose
  options the client fills from the store and whose change posts to the layer
  route with the same `hx-vals`.
  The debug panel gains the stress button. The header gains Spawn for the GM
  and My pawn for a player with a character. The tool strip from phase 6 is
  not here yet; the Select tool is a button beside the existing view controls
  and Shift-drag works without it.

## Client

### Modules

```
render/sprites.ts     SpriteCache: 256-square texture array, LRU, disc-with-initials fallback
render/pawn-pass.ts   instanced sprites: centre, half extents, layer, shape flag (disc or rect), border colour, alpha
render/ring-pass.ts   instanced condition rings, selection rings and rectangles, ghost outlines
render/path-pass.ts   highlighted cells and the path line
pawns.ts              hit testing, hover, the drag state machine, arming and placement
selection.ts          the selection set, marquee, riders lookup, may-move checks
path.ts               snapPoint (port of snap.go, per axis), supercover cells, distance under both rules
overlay.ts            the DOM overlay for one pawn or a selection
dialogs.ts            listens for room:arm, opens the spawn modal and the pawn and stat block windows from the overlay
```

### Rendering

Draw order after tiles and grid, over the viewed layer's pawns only: path
highlights for every active drag, then pawns sorted by `z` then id, then rings and selection outlines, then drag
ghosts at half alpha, then the path lines and distance labels. Labels are the
one text on the canvas; draw them with a small glyph atlas rendered once from a
2D canvas for the digits, space, `f`, `t` and `.`, so a label is a handful of
quads.

Pawn instance data lives in a preallocated `Float32Array` grown by doubling and
rebuilt only when the store's pawn list or the viewed layer changes, not per
frame; per-frame
changes (ghosts, hover, marquee) are separate small buffers. Hidden pawns for
the GM draw desaturated at 60 percent alpha. A creature at zero HP draws a
skull glyph over a greyed sprite.

A creature's radius in map pixels is `footprint * cellSize / 2`, with tiny at
a quarter cell. An object's half extents are `footprintW * cellSize / 2` by
`footprintH * cellSize / 2`, and its image is fitted by aspect inside them.
When no map is set the cell size still comes from the grid.

### Selection

`selection.ts` holds an ordered set of ids. `mayMove(pawn)` is true for the GM
always and for a player when `ownerId` is theirs. Marquee: on Shift-press or
with the Select tool active, a drag on empty space draws a rectangle in map
space; on release, every movable pawn whose centre is inside is selected,
replacing the set unless Shift is held, in which case it adds. Selection is
pruned when a pawn leaves the store.

`riders(anchor)` returns the ids of pawns on the same layer with `z` greater
than the anchor's whose centre lies inside the anchor's footprint rectangle, when the anchor's
footprint is at least two on either axis; otherwise none.

### Interaction

`pawns.ts` consumes `input.ts` events in map coordinates:

- **Hover**: hit test on pointer move when not dragging; sets the overlay
  target when nothing is selected.
- **Select**: primary click on a pawn selects it alone; Shift-click toggles it;
  click on empty space clears the selection.
- **Drag**: primary press on a movable pawn starts a drag after four device
  pixels of movement, so a click still selects. The pressed pawn is the anchor.
  The drag set is the selection if the anchor is in it, otherwise the anchor
  alone, plus `riders(anchor)` unless Alt is held, capped at `SelectionMax`.
  The anchor's ghost snaps by `snapPoint` with its own footprint when snapping
  is on; every other ghost is its committed position plus the anchor's delta.
  The path recomputes when the snapped anchor cell changes and `pawn.drag`
  sends on that change. Release sends `pawn.move` with the anchor's snapped
  point and the other ids. Escape sends `pawn.move` with the committed point.
- **Placement**: `room:arm` puts the canvas in placement mode with a crosshair
  cursor and a ghost of the armed thing following the pointer, disc or
  rectangle by kind. Each primary click sends `pawn.spawn` with the armed kind,
  id, footprint, the viewed layer, the snapped point, and the visible flag. Escape or arming
  something else exits. The mode survives a spawn so an encounter is placed
  with repeated clicks.
- **Middle button** still pans in every mode.

Other players' drags render from `pawn.dragging` when the anchor is on the
viewed layer: a ghost at each listed position, snapped locally for the anchor only, the path from the anchor's
committed position, in a colour derived from the dragging player's id (a fixed
palette indexed by a hash), and a small name label on the anchor.

### Overlay

`overlay.ts` positions the templ-rendered overlay element above the hovered
pawn or the selection's bounding box each frame while one exists, using
`worldToScreen`. For one pawn it fills name, HP text from `hp` and `maxHp` or
the band word when that is all the viewer has, AC, and a row of coloured
condition dots with names on hover. Its Details button opens the pawn WINDOW
and, for the GM on a monster pawn, a Stat block button opens the stat block
window -- both are `data-window` triggers the client fills in with the pawn's id
and name, not `modal:open`; see the last section of this document. Its Edit
button opens the edit form in the content modal. For several pawns the overlay
shows the count and, for the GM, the Remove button whose `hx-vals` the client
sets to the selected ids.

### Stress

The debug panel's stress button adds 500 synthetic pawns to a client-only list
that the pawn pass draws alongside real ones, with random kinds, sizes and
images from the pawns already loaded. The benchmark then measures with them
present. Nothing is sent to the server.

## Tests

**TypeScript**:

- `path.test.ts`: `snapPoint` for the four parity and mode cases with offsets,
  matching the Go tests' numbers, plus a two by three footprint snapping each
  axis differently; supercover cells for horizontal, diagonal and
  knight's-move lines; distances under both rules including the 5-10-5
  sequence 5, 15, 20, 30 for one to four diagonals.
- `selection.test.ts`: a marquee selects only movable pawns on the viewed
  layer whose centre is inside; Shift adds; riders ignore pawns on other
  layers; `riders` returns pawns above a wagon and not beside it,
  and nothing for a medium creature; the drag set caps at `SelectionMax`.
- `pawns.test.ts`: hit testing prefers the topmost pawn and uses a rectangle
  for objects; a drag does not start under four pixels; the delta applied to
  non-anchor ghosts equals the anchor's snapped delta; Escape sends the
  committed position with the same others.

**Go**:

- Resolution for each kind: another GM's monster is `not_found`; a player
  spawning another player's character is `forbidden`; a monster with no image
  yields an empty image; sizes normalise; an object takes the asset's name
  when blank and refuses a footprint over the limit.
- `UpdatePawnHP` arithmetic: `12` sets, `-7` subtracts, `+3` adds, clamps to
  `[0, maxHp]`, and garbage is a 422. It answers with the panel and sends no
  `HX-Trigger` that would close a modal. An object form with conditions is a
  422.
- `RemovePawns` refuses a player, caps the id count, and dispatches one
  command with every id.
- `MovePawnsToLayer` refuses a player and dispatches `pawn.setLayer` with every
  id and the layer; `UpdatePawn` dispatches `pawn.setLayer` only when the
  layer changed.
- The pawn fragment renders the Layer select for the GM and not for a player.
- The stat block route refuses a player when the pawn is hidden and reads with
  the room owner's id.
- Write-through runs `UpdateCharacterCurrentHP` for a player pawn and not for a
  monster.
- Template tests pin the pawn form's trio, that visibility controls render
  only for the GM, and that the object form has footprint fields and no
  conditions editor.

## Verification

1. `make js && make check` and the CSS selector diff.
2. GM spawns three goblins with Visible off, then a player checks: nothing on
   their canvas, nothing in their debug panel's state. GM toggles one visible;
   it appears for the player as `pawn.spawned`.
3. Player drags their own pawn with snapping on; the GM sees the ghost, path
   and distance label move cell by cell and land where the player released.
   Player presses Escape mid-drag; the GM's preview vanishes and the pawn is
   where it was.
4. GM places a wagon token as a two by four object, then Spawn party puts
   three players on it. GM drags the wagon: the three ride along with their
   offsets intact, on both screens, and land snapped with the wagon. Alt-drag
   moves the wagon out from under them.
5. GM marquees two goblins and a player pawn, drags the group; all three move
   by one delta. The player, whose marquee cannot include goblins, selects
   only their own pawn.
6. Switch the diagonal rule; the same diagonal drag reports 15 feet instead of
   10 for two cells.
7. Set monster HP to band; the player's overlay says Bloodied when the GM
   takes a goblin below half.
8. Edit a player pawn's HP to `-5`; the character sheet's current HP dropped by
   five.
9. GM selects five pawns and removes them from the overlay behind the confirm
   modal; five `pawn.removed` events arrive with consecutive sequence numbers.
10. GM views the first floor while the party is on the ground floor, spawns two
    guards there: the player sees nothing. GM makes the first floor active:
    the player's canvas crossfades and the guards appear as `pawn.spawned`,
    while their own pawn, still downstairs, leaves as `pawn.removed`. GM moves
    the party up with the overlay's Move to layer: the player's pawn comes
    back.
11. Stress: 500 pawns and the benchmark still under the phase 4 numbers plus a
    millisecond.

## Out of scope

Fog interaction with visibility, strokes, pings, initiative, pawn image
changes after spawn, rotation, elevation, auras, an explicit attach
relationship between pawns, group editing of stats, and templates for spell
areas.

---

## Windows, and how live data reaches them

Added after phase 3, which built the window system and the player list in it.
Read the Windows section of `CLAUDE.md` first; this is what it means for the
surfaces above, and it **supersedes** decision 10's content modal for the pawn
panel and the stat block's row in the route table.

### Which surface is which

A window is not a better modal, and picking between them is one question: does
this stay open while the GM works, or is it a task they finish?

| Surface | Kind | Why |
| --- | --- | --- |
| Pawn panel: name, HP, AC, conditions, layer | **Window**, `pawn:<ulid>` | It stays open through a fight, several at once, tucked in a corner. This is what the old client opened on right-click and it is the interaction worth carrying forward. |
| Stat block | **Window**, `monster:<ulid>` | Keyed by the monster and not the pawn, so eight goblins share one window rather than opening eight identical ones. |
| Edit form: rename, size or footprint, max HP, visibility | Content modal | A task with a Save. A form that refetched itself would throw away what somebody was typing; see below for the one field that is the exception. |
| Spawn dialog | Content modal | A task with an outcome: it arms placement and closes. |

The pawn panel's trigger lives on the selection overlay rather than in a menu,
because a pawn window is about one pawn and the overlay is where that pawn is
named:

```go
RoomWindow{
    ID:     "pawn:" + pawn.ID.String(),
    Title:  pawn.Name,
    URL:    "/fragment/room/pawn?room=" + roomID + "&pawn=" + pawn.ID.String(),
    Width:  260,
    Height: 300,
}
```

### How a window stays live

**The socket says what changed; the GET fetches what it looks like.** The
fragment reads the hub's in-memory room, not the database, so the refetch is a
render of the same state the socket event came from -- there is no second source
of truth and no query behind it.

Two things make it scale to a table full of open pawn windows.

**The DOM event carries the id.** `panels.ts` maps `pawn.updated`, `pawn.spawned`
and `pawn.removed` to a `room:pawn` event with `detail: { id }`. Today its
events carry no detail at all, because nothing consumes one; this is a two-line
change when the first pawn window lands.

**The fragment filters on it in its own markup.** htmx's trigger filter is a
condition in brackets after the event name, evaluated with the event's
properties bound as variables and `this` bound to the element:

```html
<article id="pawn-01H..."
    hx-get="/fragment/room/pawn?room=...&pawn=01H..."
    hx-trigger="room:pawn[detail.id === '01H...' && !this.contains(document.activeElement)] from:window"
    hx-sync="this:queue last"
    hx-swap="outerHTML">
```

Ten pawn windows open and one goblin taking damage is one GET. Without the
filter it is ten.

`hx-sync="this:queue last"` is not optional and is the same rule the player
window follows: a burst of events is a burst of GETs whose answers can land in
either order, and the socket being ordered does not make a pair of HTTP requests
ordered. Never `replace` -- it cancels the request in flight and htmx reports
every cancellation as a console error.

**`document.activeElement` is what keeps a refetch from eating a keystroke.**
The one editable control in the panel is HP, because that is the value that
changes every round and a modal in front of "the goblin takes 7" is the wrong
shape. The filter above skips the refetch entirely while focus is anywhere
inside the panel, and the swap that follows the person's own POST brings the
panel back in step. Every other edit is the content modal's form, which cannot
be clobbered because nothing refetches it.

### What the server owes this

`hub.Pawn(ctx, roomID, pawnID, role)` beside the existing `Players`, returning
the pawn **projected for the requesting role** -- the same `projectPawn` the
socket calls.

**THIS IS THE ONE THING IN THE PHASE THAT MUST NOT BE GOT WRONG.** The whole
two-audience design says a hidden pawn never reaches a player's browser. An
unprojected fragment handler is a back door around it: a player guesses a ULID,
GETs the fragment, and reads a hidden monster's exact hit points that the socket
was careful never to send. The route table above already says "404 empty if
absent or hidden from the requester" and that clause is load-bearing. The test
for it is in the list below and should be written first.

The same applies to the stat block route, which is already scoped that way.

### The rule about which events may drive a refetch

**Only events that count, and never a hot path.** Hit points, armour class,
conditions, name and layer change at human pace -- a few per round -- and a GET
each is nothing.

`pawn.moved`, `pawn.dragging` and `stroke.extended` are the three hot paths in
the protocol. They fire up to twenty times a second, and binding a refetch to
one of them is precisely what "the socket carries JSON and the DOM refetches"
was designed to avoid. A window that wants a per-frame value does not fetch it:
the client already holds the whole projected room in `store.ts`, which is what
the debug panel reads. htmx renders the structure once and a small binding layer
updates a few marked spans in place. That is roughly thirty lines and it is not
worth writing until something needs it; the point of recording it here is that
the escape hatch exists and does not require touching the protocol.

### Two loose ends the window creates

**A removed pawn leaves a stale window.** The fragment 404s, `noSwap` covers
4xx, so htmx swaps nothing and the panel sits there showing a dead goblin's hit
points forever. `panels.ts` dispatches `window:close` with the id on
`pawn.removed`, and the window manager closes it. Build it with the first pawn
window rather than finding it at a table.

**A renamed pawn keeps its old title.** A window's title is set from the trigger
when it opens, so `pawn.updated` with a new name does not reach it. The fix is
the mirror of the above: `window:retitle` with an id and a title, and one line
in the manager that sets the heading's `textContent`.

### Responsiveness inside a window

The panel is 260px wide by default and can be dragged to 900. **Style it with
container queries, and let the fragment declare its own container** -- not the
window:

```html
<article class="@container">
    <dl class="grid grid-cols-1 @sm:grid-cols-2 gap-2">
```

Tailwind 4 has these in core (verified against the vendored 4.3.3 binary:
`@container`, named containers, `@min-[420px]:` and `cqi` units all emit). The
reason the container belongs to the fragment is that a container query with no
query container above it evaluates to *unknown*, which is false -- so a fragment
that relied on the window declaring one would render permanently in its narrow
state anywhere else, including in the content modal and on a full page. A
fragment that carries its own container is sized by its own box everywhere.

`container-type: inline-size` queries width only. Height queries need
`container-type: size`, which needs a definite height; a window body has one and
a page section usually does not.

### Tests this section adds

**Go**:

- `hub.Pawn` projects: a player asking for a hidden pawn gets nothing, and a
  player asking for a visible monster with the band setting gets a band and no
  numbers. This is the security test and it is the first one to write.
- The pawn fragment renders its `hx-trigger` with the pawn's own id in the
  filter and with `hx-sync="this:queue last"`, so a refactor cannot quietly turn
  ten windows into ten GETs or reintroduce the ordering race.

**TypeScript**:

- `panels.test.ts`: `pawn.updated` dispatches `room:pawn` carrying the pawn id;
  `pawn.removed` dispatches both that and `window:close`; none of
  `pawn.moved`, `pawn.dragging` or `stroke.extended` dispatches anything at all.
  That last assertion is the hot-path rule, written down where it fails.

### Verification this section adds

12. Open the pawn panel for two goblins. The GM takes seven off the first: its
    window updates and the second window makes no request (check the network
    panel -- one GET, not two).
13. Click into the first window's HP field and type. The GM's other tab changes
    that pawn's AC; the field keeps what was typed. Blur it, change the AC
    again, and the panel catches up.
14. A player opens a panel for a visible monster with hit points set to band:
    it says Bloodied and no numbers. The GM hides that monster; the player's
    window is closed by `pawn.removed` rather than left showing stale stats.
15. Drag the pawn panel to 900px wide: the stat rows go from one column to two
    without a reload.
