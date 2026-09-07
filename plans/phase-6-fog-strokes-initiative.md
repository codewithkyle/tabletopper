# Phase 6: fog, strokes, pings and initiative

Read `plans/vtt-overview.md`, `plans/phase-2-protocol-core.md` for the fog,
stroke, ping and initiative commands, and `plans/phase-5-pawns.md` for the
render passes and interaction modes this extends. This phase finishes the
table's first feature set: the GM hides and reveals the map, anyone allowed
draws on it, anyone pings it, and the initiative tracker runs the fight.

## End state

- The GM turns fog on. The map is covered for players and tinted for the GM.
  Rectangle and polygon tools reveal or hide areas; Escape cancels a shape,
  Enter closes a polygon. Fog shapes can be removed one at a time from a list
  or cleared.
- Anyone the table allows draws freehand strokes in one of eight colours at a
  chosen width, erases whole strokes by touching them, undoes their own last
  stroke, and the GM clears everything. Strokes appear on other clients as
  they are drawn.
- Anyone pings a point. Every client sees a two-second ring in the pinger's
  colour and hears a short sound unless they muted pings for themselves.
- The initiative panel lists entries with portrait, name and a number. The GM
  adds pawns, sets numbers, sorts, reorders, advances and clears. The active
  entry is highlighted everywhere; the player whose pawn is active sees a
  turn timer and an End turn button; conditions with durations tick down as
  turns pass.

## Decisions

1. **Fog shapes are the source of truth; the mask is a cache.** Fog is per
   layer: each layer has its own enabled and prefill flags in the table, and
   each shape carries its layer. Each client rasterises the viewed layer's
   shapes into an offscreen framebuffer texture covering that layer's map at a
   quarter of native resolution, capped at 4096 on the long side. A new shape
   on the viewed layer is drawn onto the existing mask; a removal, clear,
   prefill change or a change of viewed layer rebuilds it. Polygons are triangulated with `earcut`, added as the
   first npm runtime dependency of the room bundle.
2. **The fog quad is drawn after pawns for players and before them for the GM.**
   A player must not see a pawn that stands in fog, so for players the fog
   covers pawns. The GM sees the tint under the pawns. This is client-side
   concealment: the pawn's data has already reached the player. Server-side
   fog-aware projection, which needs a point-in-polygon pass per fog change,
   is deferred and listed as a hardening item in the overview's open
   questions.
3. **Fog colour follows the theme.** The player's fog is the page's base
   colour at full alpha; the GM's is the same colour at half alpha over the
   map. Both re-read on `theme:change`.
4. **Strokes render as instanced segments with round caps in the shader.** Each
   segment instance carries its two endpoints, width and colour; the fragment
   shader computes the distance to the segment and produces a round-capped,
   anti-aliased line. One draw for every finished stroke from a buffer that
   only grows when a stroke ends, and one small draw for strokes in progress.
   A few hundred thousand segments is an ordinary frame.
5. **Erasing removes whole strokes.** The eraser tests the pointer against
   stroke segments on the CPU and sends `stroke.erase` with the ids it
   touched, batched per pointer-up. A pixel eraser would make the texture the
   truth and lose undo; it is not built.
6. **Points are decimated on input.** With `getCoalescedEvents`, a point is
   kept only when it is at least one map pixel from the last kept point at the
   current zoom, and chunks go out every 100 milliseconds or 64 points,
   whichever comes first. Strokes stay under the core's limits at any drawing
   speed.
7. **Ping sound is a client preference in `localStorage`**, wrapped in try and
   catch, default on. The old per-player mute was a server feature; a viewer
   deciding what they hear is a viewer's setting.
8. **Initiative controls are HTTP forms and buttons in the panel**; the panel
   refetches on `initiative.updated` as phase 3 set up. Only End turn from the
   turn timer goes over the socket, because it is the one control a player
   uses and it should feel instant.
9. **Player colours come from one palette indexed by a hash of the player id**,
   shared by drag ghosts, pings, in-progress strokes and the members list.
   Eight colours, the same eight the stroke tools offer.

## Server

### Routes

| Pattern | Wrapper | Handler | Behaviour |
| --- | --- | --- | --- |
| `GET /fragment/room/fog?room={id}&layer={id}` | `Fragment` | `RoomFogFragment` | GM only, for one layer (the client passes the viewed layer). Its name as the heading, toggles for enabled and prefill, the list of that layer's shapes with kind, mode, point count and a Remove button each, a Clear all button behind `hx-confirm`, Close. |
| `POST /rooms/{id}/layers/{layer}/fog` | `RequireSession` | `SetLayerFog` | GM only. Reads `enabled` and `prefill`, dispatches `fog.setEnabled` and `fog.setPrefill` for the layer. Returns the re-rendered toggles. |
| `DELETE /rooms/{id}/fog/{shape}` | `RequireSession` | `RemoveFogShape` | GM only. Dispatches `fog.remove`. Returns nothing; the row is `hx-swap="delete"`. |
| `DELETE /rooms/{id}/layers/{layer}/fog` | `RequireSession` | `ClearLayerFog` | GM only. Dispatches `fog.clear` for the layer. `htmx.CloseModal`. |
| `DELETE /rooms/{id}/layers/{layer}/strokes` | `RequireSession` | `ClearLayerStrokes` | GM only, behind `hx-confirm` on the toolbar button, which the client points at the viewed layer. Dispatches `stroke.clear` for the layer. |
| `GET /fragment/room/initiative?room={id}` | `Fragment` | `RoomInitiativeFragment` | Any member. Renders the panel from `hub.Initiative(roomID)` projected for the requester's role, with GM controls for the GM. |
| `GET /fragment/room/initiative/add?room={id}` | `Fragment` | `AddInitiativeFragment` | GM only. Pick list of pawns not yet in the tracker, each with a number field defaulting to blank, plus a free-text row for a named entry. |
| `POST /rooms/{id}/initiative` | `RequireSession` | `SetInitiative` | GM only. Receives the whole entry list from either the add dialog or the panel's reorder and number edits, dispatches `initiative.set`. |
| `POST /rooms/{id}/initiative/next` | `RequireSession` | `NextInitiative` | GM or the active pawn's owner; the core enforces it. Dispatches `initiative.next`. |
| `POST /rooms/{id}/initiative/sort` | `RequireSession` | `SortInitiative` | GM only. Reads the live entries, sorts by number descending with stable ties, dispatches `initiative.set` keeping `active` on the same entry. |
| `DELETE /rooms/{id}/initiative` | `RequireSession` | `ClearInitiative` | GM only, behind `hx-confirm`. Dispatches `initiative.clear`. |

The panel's per-entry controls are Up, Down and Remove buttons that post the
full reordered list to the set route, and a number input that posts on change.
This is more requests than a drag-to-reorder widget and far less code; the
tracker has a dozen entries.

`hub.Initiative` and `hub.Fog` are two more read accessors beside `Table` and
`Pawn` from earlier phases, each returning a copy taken on the actor's
goroutine through a `dispatch`-style request.

### Templates

- `pages/room-fog.templ`, `.go`: the fog dialog.
- `pages/room-initiative.templ`, `.go`: `RoomInitiativeFragment(data
  InitiativeData)` with the entry rows, the active highlight, GM controls, and
  `AddInitiativeFragment`. The turn timer and End turn button are in the panel
  markup, hidden, and shown by the client when it is the viewer's turn.
- The room page gains a tool strip for the canvas: Move, Measure, Fog
  (GM), Draw, Erase, Ping, with the draw colour swatches and a width range
  input rendered in templ and read by the client. Keyboard shortcuts `v`,
  `m`, `f`, `d`, `e`, `p` select tools; Escape returns to Move. The strip's
  buttons carry `data-tool` attributes; the client toggles an `aria-pressed`
  attribute and the templ styles it.
- The side column gains the initiative panel with its `hx-trigger`, and the
  fog dialog and clear-strokes controls join the header for the GM.

## Client

### Modules

```
render/fog-pass.ts     the mask framebuffer, shape rasterisation, the fog quad for each role
render/stroke-pass.ts  segment instancing for finished and in-progress strokes
render/ping-pass.ts    expanding rings with a two-second life
render/text.ts         the glyph atlas from phase 5, extended with letters for the distance label and names
tools.ts               the tool state machine: move, measure, fog-rect, fog-poly, draw, erase, ping
fog.ts                 shape construction, preview geometry, earcut triangulation
strokes.ts             capture, decimation, chunking, erase hit testing, undo
pings.ts               local effects: ring, sound, mute preference
initiative.ts          turn timer, End turn, turn-start toast
```

### Fog

Rendering the mask, for the viewed layer: clear to that layer's prefill
(covered or not), then for each of its shapes in order draw its triangles with a colour of 1 for reveal or 0 for hide into a
single-channel texture. `fog-pass.ts` keeps the mask texture and a count of
shapes drawn; an `fog.added` with the count matching draws only the new shape;
anything else rebuilds. The fog quad samples the mask at the map coordinate and
outputs the fog colour where the mask is 0.

Tools: fog-rect drags a rectangle with a live preview; fog-poly clicks points
with a rubber-band preview, Enter closes with at least three points, Escape
cancels. A Reveal or Hide toggle in the tool strip sets the mode. Both send
`fog.add` on completion with the viewed layer and integer map-space points.

### Strokes

Draw: pointer down begins a stroke with a client-minted ULID (the bundle needs
a small ULID function: 48-bit time plus 80 random bits, Crockford base32),
sends `stroke.begin` with the viewed layer and the first chunk, `stroke.extend` per chunk,
`stroke.end` on release. The local stroke draws from the local points
immediately; the echo is ignored for the own stroke until `stroke.ended`, at
which point the store's copy replaces the local one.

Erase: pointer down starts collecting; each pointer move tests the pointer
against every segment of every stroke within a bounding-box prefilter and
marks hits; pointer up sends one `stroke.erase`. Own strokes only for a
player; anything for the GM.

Undo: `Ctrl+Z` in Draw or Erase mode sends `stroke.erase` for the viewer's
most recent finished stroke.

Segment buffers: finished strokes on the viewed layer live in a `Float32Array`
of instances that is appended on `stroke.ended` for that layer and rebuilt on
`stroke.erased`, `stroke.cleared` or a change of viewed layer. In-progress strokes, own and others', live in a second small
buffer rebuilt per frame while any exists.

### Pings

A `pinged` event on the viewed layer, or the local Ping tool click, adds a
ring at the point with the player's colour and a two-second life; a ping on
another layer is ignored. The Ping tool sends the viewed layer. Rings animate from a quarter cell to
two cells in radius while fading; the frame loop stays alive while any ring
lives. The sound is a short clip at `/static/ping.mp3` at volume 0.25, played
through one `Audio` element reused across pings, skipped when muted. A speaker
button in the tool strip toggles the preference.

### Initiative

`initiative.ts` watches the store: when `initiative.active` changes to an entry
whose pawn the viewer owns, it shows the turn timer, starts a count-up in
`MM:SS` with the colour steps from the old client (safe under thirty seconds,
warning under a minute, danger after), raises a toast "Your turn", and shows
End turn. End turn sends `initiative.next` over the socket. Any other change
hides the timer. The panel itself refetches on `room:initiative`.

### Measure

The Measure tool from the old client comes along cheaply: drag from a point,
draw the path and distance label with the phase 5 code, nothing sent.

## Tests

**TypeScript**:

- `fog.test.ts`: shapes on other layers are excluded from the mask; a viewed
  layer change triggers a rebuild; a rectangle produces two triangles in map
  coordinates; a
  concave polygon triangulates without degenerate triangles; a shape with a
  duplicate closing point is accepted.
- `strokes.test.ts`: decimation drops points closer than the threshold; chunks
  never exceed 64 points; erase hit testing finds a segment within half the
  width plus the eraser radius and not outside it; the ULID function produces
  26 Crockford characters that sort by time.
- `initiative.test.ts`: the timer colour steps at the boundaries; End turn is
  offered only when the active entry's pawn is owned by the viewer.

**Go**:

- Fog routes dispatch the right commands for the layer in the path; a player
  is refused everywhere; the fog fragment lists only that layer's shapes.
- `SortInitiative` orders descending, keeps ties stable, and preserves the
  active entry by id.
- `NextInitiative` from a player who does not own the active pawn is
  `forbidden` from the core and rendered as an alert.
- The initiative fragment projected for a player omits entries of hidden
  pawns.
- Template tests pin the panel's `hx-trigger` and that GM controls render only
  for the GM.

## Verification

1. `make js && make check` and the CSS selector diff.
2. GM enables fog with prefill; the player's canvas is a solid block. GM
   reveals a rectangle and a polygon; the player sees the map there and
   nothing else, and no pawn outside the reveal. GM hides part of the reveal
   with a hide polygon; it closes back up. Remove a shape from the list;
   the mask rebuilds correctly.
3. Player draws a long fast stroke; the GM sees it appear in chunks and finish
   with round caps and no gaps. Player erases it by touching it; gone on both.
   GM disables players can draw; the player's Draw tool is refused with an
   alert.
4. Player pings; both hear the sound; the player mutes and pings again;
   only the GM hears it.
5. GM adds four pawns to initiative with numbers, sorts, advances twice; the
   player whose pawn is second sees the timer and toast on their turn, ends
   it, and a condition set to two turns clear-on-end on that pawn shows one
   turn left.
6. Fog the ground floor, draw on it, then make the first floor active: the
   player sees an unfogged, undrawn first floor. Switch back: the fog and
   drawings are exactly as left.
7. Benchmark with the fog mask on, a thousand strokes and the stress pawns;
   record the numbers against phase 5.

## Out of scope

Server-side fog projection, walls and dynamic lighting, spell templates,
dice, chat, music playback in the room, a turn-order roll button, per-player
ping mute enforced by the server, and drag-to-reorder for initiative.
