# Phase 5: pawns

Read `plans/vtt-overview.md`, then `plans/phase-2-protocol-core.md` for the
pawn commands, the object kind and per-axis snapping, `plans/phase-3-transport.md`
for `Dispatch` and resolution, and `plans/phase-4-renderer.md` for the render
passes this adds to. This phase puts creatures and objects on the table:
spawning from the monster manual, the token library and the party's
characters; selecting one or many; moving them with snapping and the shared
movement path; and editing them through a dialog.

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
10. **The pawn dialog is a fragment, and its form posts over HTTP.** The
    handler builds `pawn.update`, `pawn.setConditions` and `pawn.setVisible`
    as needed and dispatches them. The HP field accepts `12`, `-7` or `+3`,
    evaluated server-side: an absolute value replaces, a signed value adds to
    the current. Errors from the core come back as 422 form errors.
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
| `GET /fragment/room/pawn?room={id}&pawn={id}` | `Fragment` | `RoomPawnFragment` | Owner or GM of the pawn. Reads the live pawn via `hub.Pawn(roomID, pawnID)`; 404 empty if absent or hidden from the requester. Renders the edit form; objects get footprint fields in place of size and no conditions editor; the GM also gets a Layer select listing the room's layers from the live table, current layer selected. |
| `POST /rooms/{id}/pawns/{pawn}` | `RequireSession` | `UpdatePawn` | Parses the form, evaluates HP arithmetic against the live value, dispatches `pawn.update`, then `pawn.setConditions` when the repeater changed, then `pawn.setVisible` when the GM toggled it, then `pawn.setLayer` when the GM changed the layer. `htmx.CloseModal`. |
| `POST /rooms/{id}/pawns/layer` | `RequireSession` | `MovePawnsToLayer` | GM only. Reads repeated `ids` and `layer`, at most `SelectionMax`, dispatches `pawn.setLayer`. Empty 200. Used by the pawn dialog with one id and by the overlay with the selection. |
| `DELETE /rooms/{id}/pawns` | `RequireSession` | `RemovePawns` | GM only. Reads repeated `ids` form values, at most `SelectionMax`, dispatches one `pawn.remove`. Empty 200. Used by the dialog with one id and by the overlay with the selection. |
| `GET /fragment/room/stat-block?room={id}&pawn={id}` | `Fragment` | `RoomStatBlockFragment` | GM, or any member when the pawn is visible. Resolves the pawn's `monsterId`, reads with the room owner as owner, renders the same stat block component the manual uses. |

The Spawn and pawn dialogs open in the content modal at size `lg`. Buttons are
solid `btn`; Close first.

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
dialogs.ts            listens for room:arm and room:view, opens the pawn dialog and stat block from the overlay
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
condition dots with names on hover; its Edit button opens the pawn fragment
through `modal:open`, and for the GM and a monster pawn a Stat block button
opens the stat block fragment. For several pawns it shows the count and, for
the GM, the Remove button whose `hx-vals` the client sets to the selected ids.

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
- `UpdatePawn` HP arithmetic: `12` sets, `-7` subtracts, `+3` adds, clamps to
  `[0, maxHp]`, and garbage is a 422. An object form with conditions is a 422.
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
