# Virtual tabletop: architecture overview

Decided 2026-09-07 in a design discussion before any code was written. This is
the top-level spec for the VTT. Detailed specs for the protocol catalog, the
renderer, and room persistence sit beside it when they are written and must
agree with what is here. Where they conflict, fix this document first.

The old client under `./client` is a behavioural reference only. Nothing in it
is carried forward: not the web components, not the uWebSockets protocol, not
the single-texture renderer. Its roughly forty room event types are useful as a
feature checklist and nothing more.

## Goals

- Performance and quality over feature count. Every decision below was made by
  asking what the actual cost is, and choosing the simplest design that stays
  well inside it.
- One Go process owns everything: pages, fragments, rooms, sockets. The VTT is
  not a separate service and not a separate deploy.
- Server-rendered shell, canvas-rendered table. The DB-backed features the app
  already has (character sheets, monster manual, journals, assets) keep their
  routes and are composed into the room page, not re-implemented inside it.

## Non-goals

- Horizontal scaling. Room state lives in one process and the design assumes
  it. If that ever changes, the answer is sticky routing by room code, not a
  shared store. Do not build for it now.
- Dynamic lighting, walls, or line-of-sight. Nothing here precludes them, and
  they are the one class of feature in this genre with real computational
  weight. They are deferred, not designed.

---

## Topology

### Rooms are actors in the Go process

One goroutine owns each live room and is the only code that reads or writes
its state. Clients send commands into the room's inbound channel. The room
validates, applies, and broadcasts. There are no locks on room state because
there is no shared access to it.

Room state is in memory only, and holds:

- **Table**: an ordered list of layers, each a named slot holding at most one
  map (asset ULID, tile generation, width, height, tile size, max zoom) and its
  own fog flags; which layer is active; and the room-wide grid (colour, cell
  size, x/y offset, snap mode, feet per cell, diagonal rule, whether the grid
  is drawn).
- **Pawns**: id, layer, name, x/y/z in map pixels, size or a rectangular
  footprint for objects, visible, hp, max hp, ac, conditions, owner player id,
  optional monster or character ULID.
- **Players**: id, user id, character id, name, avatar, role, connected.
- **Initiative**: entries, active entry, round.
- **Fog**: a collection of rectangle and polygon shapes, each on one layer.
- **Strokes**: a collection of drawn polylines, each on one layer, with colour,
  width, and a flat integer point array.
- **Room**: locked flag and settings.

Pawns spawned from the monster manual hold the monster's ULID plus their own
instance stats in room state. The manual is read at spawn and when a stat block
is opened. The VTT writes nothing to the monsters table. This was decided
during the monster manual design and is restated here so it is not reopened.

### The rooms table

`rooms` has existed since migration `20260227175838` but has never been
written. Phase 1 rebuilds it: id, owner, name, a nullable four-character code
(NULL once closed, unique otherwise), a locked flag, the three snapshot
columns, timestamps, closed-at. Open means `closed_at IS NULL`; there is no
separate open flag. `sessions` already carries `room_id` and `character_id`
and they become the membership record.

The row means "this room exists, here is who owns it and how to join it". It
does **not** record whether the room is running. Liveness is a question for the
in-process hub. A running flag in the database goes stale the first time the
process dies and every join would have to distrust it anyway.

### Snapshots

In-memory state is not durable, and deploys restart the process in the middle
of Saturday's game. So the room writes a debounced JSON snapshot of its full
state a few seconds after its last change. On SIGTERM every live room
snapshots synchronously before the server drains; `main.go` already has the
context for this. On the first join after a restart, the hub rehydrates the
room from its snapshot.

This turns a deploy into a "reconnected" toast instead of a lost session, for
one query per few seconds per active room. The snapshot is three columns on
the `rooms` row (`snapshot`, `snapshot_seq`, `snapshot_at`), decided in phase
1, and it is the same marshaller that encodes the snapshot sent to a
connecting client.

### Connections

- The socket is same-origin, so the existing session cookie authenticates the
  upgrade. The handler checks the `Origin` header. The route is a resource
  route under `/rooms/{code}/...`, not a fragment; it does not return HTML.
- Each connected client has one buffered outbound channel. The room does a
  non-blocking send into each. A full buffer means a slow client, and the
  policy is to close that client; it reconnects and receives a fresh snapshot.
  This is the entire backpressure story and it is decided up front.
- Reconnect always gets a full snapshot. Room state is tens of kilobytes at
  most. No replay log, no partial catch-up.
- Library: `github.com/coder/websocket`. Pure Go, context on every call, small
  JSON helper. Cloudflare proxies WebSockets without configuration.

---

## Wire format

**JSON text frames.** The busiest moment of a six-player session is single-digit
kilobytes per second, and V8 parses JSON natively at hundreds of megabytes per
second. There is no bottleneck to remove, and text frames are readable in
devtools, which matters more than anything else while a new VTT is being
built. JSON is also the snapshot encoding, so one marshaller serves the wire
and the database.

Rejected: MessagePack and CBOR (a quarter smaller, unreadable, a library on
both sides, nothing else); Protobuf (its real benefit is a shared schema, which
we get for less, see below); custom binary (only ever for a hot path that
measurement has shown to be hot, and none has).

Rules:

- **Flat envelope.** Events carry `type` and `seq`, plus `by` when a player
  caused them. Commands carry `type` and a client correlation id `cid`.
  Payload fields sit beside these at the top level, with entities as named
  objects. Go decodes `type` first, then the concrete struct from the same
  bytes.
- **Go owns the types and generates the TypeScript.** The server validates
  every message, so its structs are the authority. A go-generate step emits
  `server/js/room/protocol.ts` from them, beside sqlc and templ; `make
  protocol` runs it and a test fails when the committed file is stale. Two
  hand-maintained type sets drift, and drift is the bug found at the table.
  It is a `.ts` module rather than the `.d.ts` this document first said,
  because the generated file carries one runtime value as well as types --
  the set of transient event names, which the client's dispatch splits on --
  and a declaration file cannot hold it. tygo was considered and rejected in
  phase 2: it emits interfaces but cannot emit the two discriminated unions,
  which are built from the type strings in the command and event registries.
- **Server-assigned sequence, one counter per audience.** Every event that
  changes state carries a monotonic `seq`, stamped by the hub, and there are
  two counters -- GM and players -- because a single room-wide one would number
  the GM-only events into the players' stream and put holes in it by design.
  Per connection would work too and would cost the encode-once-per-audience
  property the whole projection exists for. Per audience is exact because of a
  property of the catalog: every event that is not transient goes to a WHOLE
  audience, and the three partial audiences carry only transient events.
  Transient events and the snapshot carry the current value without advancing
  it, so a client checks `seq == last + 1` over exactly the events its reducer
  runs, and a gap means the server dropped something rather than that the
  client was not addressed. Clients never stamp their own timestamps, and
  phase 2's reducer deliberately does not track `seq` at all, so state and
  sequence cannot get tangled together.
- **Strict decoding and hard limits.** Unknown fields are rejected. Frames are
  capped at a few tens of kilobytes; strokes are chunked to stay far under it.
  Per-client message rate is capped.
- **Integers everywhere.** Coordinates are map pixels. Stroke points are
  quantised to integers, which also makes them shorter as text.
- **Snapshot over the socket**, as the first message after connect. No
  per-message compression: deflate on hundred-byte frames costs CPU for
  nothing, and the snapshot is the only frame big enough to benefit.
- **The binary door stays open.** A WebSocket frame is text or binary and the
  client dispatches on that before parsing. If stroke chunks ever measure as
  a problem they become a binary frame of int16 pairs without touching any
  other message. This is not expected to happen.

---

## Protocol design

### Commands in, events out

A client sends an imperative **command** (`pawn.move`). The server validates
it, applies snapping and authority, mutates the room, and broadcasts a
past-tense **event** (`pawn.moved`). The event is the server's version, which
may differ from what was asked: snapped, resolved against the monster manual,
or rejected. A command is never rebroadcast verbatim.

In TypeScript this is two discriminated unions, one emitted and one reduced,
both generated from Go. Naming is one flat namespace, `noun.verb` for commands
and `noun.verbed` for events. The old nested colon paths are gone.

### Three sizing rules

Every event payload is sized by exactly one of these:

1. **Singletons are replaced whole.** Table configuration and initiative are
   each one object. When either changes the event carries the entire new
   object. The reducer has no partial-update semantics and no null-versus-
   absent question.
2. **Collections get per-item events carrying the full item.** Pawns, fog
   shapes, strokes, and players get added, updated, and removed events. An
   updated pawn event carries the whole pawn after the change, not the field
   that changed. A pawn is a few hundred bytes. The client converges on the
   server's truth with one assignment by id.
3. **Hot paths carry deltas.** Movement, drag previews, and stroke chunks are
   frequent, so they carry only ids and coordinates: a list of pawn positions
   for movement and drags, a run of points for strokes. These are the only
   non-idempotent events, and there are three of them.

The property these rules buy: applying the snapshot and then every event in
sequence produces the same state as a fresh snapshot. Full-entity events make
that true by construction. The old protocol's eight per-field pawn events made
it something to test for and eventually break.

### Catalog

Events first, commands in the same line.

- **sync.** `snapshot` is the first message after connect and the answer to
  `sync.request`. It carries the full projected room state, the current `seq`,
  the receiving player's id and role, and the server build version. A client
  whose bundle version differs reloads.
- **room.** `updated` for lock state and settings. `closed` tells everyone to
  leave.
- **player.** `joined`, `updated`, `left`. A disconnect is `updated` with
  connected false; the player stays in state so reconnecting resumes them.
  `kicked` goes only to the kicked player.
- **table.** `updated` with the whole table object. Commands: add, remove,
  rename and reorder layers, set or clear a layer's map, set the active layer,
  set grid, set options (monster HP visibility, players can draw). GM only.
  Removing a layer deletes the pawns, fog and strokes on it; the GM confirms
  with the pawn count in front of them.
- **pawn.** `spawned`, `updated`, `moved`, `dragging`, `removed`. Commands:
  spawn, update, move, drag, set visibility, remove. Spawn from a monster
  carries the monster ULID and the server emits the resolved pawn. Update takes
  a patch of plain fields (name, hp, max hp, ac, size, layer) and the event
  returns the full pawn; set conditions replaces the condition list. Spawn
  party places a pawn for every connected player with a character. Authority
  for update, set conditions and move is GM or owner. Move and drag carry an
  anchor, its new position and the other selected pawns; the server snaps the
  anchor and moves the rest by the same delta, so a wagon and its riders move
  as one. Pawns are creatures with one of six sizes, or objects with a
  rectangular footprint in cells. Set layer moves pawns between floors and is
  GM only.
- **initiative.** `updated` with the whole tracker. Commands: set, next,
  clear. "Your turn" is derived client-side when the active entry becomes the
  player's pawn.
- **fog.** `added` and `removed` per shape, `cleared` per layer. The enabled
  and prefill flags live on the layer and ride in `table.updated`. Shapes are
  a collection because hundreds of polygons are too large to replace on every
  add.
- **stroke.** `began`, `extended`, `ended`, `erased`, `cleared`. The client
  generates the stroke id (a ULID) because it references the stroke in chunks
  before the server has answered. The server validates format and uniqueness.
- **ping.** `pinged` with a position and the actor.
- **error.** Sent only to the sender: `cid`, a short code (`forbidden`,
  `not_found`, `invalid`, `rate_limited`, `locked`), and a heading and message
  the client hands to the alert modal.

The old announcement family (`room:announce:*`) does not exist. "Player
disconnected" is a toast the client derives from `player.updated`. Lock and
unlock toasts derive from `room.updated`.

### Transient versus state

`pinged`, `dragging`, `error`, `kicked`, and `closed` are transient: they never
touch the reducer and are handled by an effects layer. Everything else mutates
state and must be reflected in the snapshot. The distinction is marked in the
type definitions so nobody adds a state event that skips the snapshot.

### Two audiences

A hidden pawn must not reach a player's browser at all. The old app sent it
with a hidden flag and trusted the client, which is a cheat that costs one
devtools tab.

The room projects every event and every snapshot for two audiences, **GM** and
**players**, encodes each once, and fans out. A pawn is **shown** to
players when it is visible and on the active layer. Hiding a pawn, or moving it
to another layer, or the active layer moving away from it, is `pawn.updated`
to the GM and `pawn.removed` to players. The reverse is `pawn.spawned` to
players. A monster's hit points and armour class project to players as a word,
or as nothing at all, per a table setting. Retrofitting this means touching every event, so it is in
the first version.

### Authorize, then apply

Every command type implements two methods: one checks the acting player
against the room and returns `forbidden` or `invalid`; the other mutates state
and returns the events to emit. The whole protocol is then testable as pure
functions from (state, command) to (state, events), with no socket in the test.

### Echo everything except drag

The sender receives its own `moved` event like everyone else, so its state
converges from the same source. Optimistic local movement of your own pawn is
fine: the echo carries the same snapped value, and if the server corrected it
the pawn slides to the truth. `dragging` is the exception and goes to everyone
except the sender, who is already drawing it.

### Coalesce per drag

The room keeps at most one pending drag per anchor pawn and flushes on a
short tick, so a fast mouse costs no more bandwidth than a slow one. With
snapping on, the client only sends a drag when the hovered cell changes, so the
hot path quantises itself.

### Example frames

```json
{"type":"pawn.move","cid":"a9","id":"01J...","x":1344,"y":832}

{"type":"pawn.moved","seq":4821,"by":"01H...","id":"01J...","x":1344,"y":832}

{"type":"pawn.updated","seq":4822,"by":"01H...","pawn":{"id":"01J...","name":"Goblin","x":1344,"y":832,"z":1,"hp":5,"maxHp":7,"ac":15,"size":"small","visible":true,"conditions":["prone"],"monsterId":"01G...","ownerId":null}}

{"type":"error","cid":"a9","code":"forbidden","heading":"Not your pawn","message":"Only the GM can move that."}
```

The same `pawn.updated` on the player audience, were the goblin hidden:

```json
{"type":"pawn.removed","seq":4822,"id":"01J..."}
```

---

## Client

### Page composition

The room page is a templ page like any other. The line between DOM and canvas
is drawn by coordinate space:

- **Canvas owns map space**: tiles, grid, fog, pawn sprites, pings, doodles,
  the movement path, measurement. Anything that moves when the camera moves.
- **DOM owns screen space**: the menu bar, the tool pill, the initiative
  tracker, the stat block modal, character panels and settings. All templ, all
  htmx, all the existing modal rules apply unchanged.
- **The room's chrome is a menu bar and a floating tool pill**, decided
  2026-09-07 after phase 1's first shell was built and rejected. A thin bar
  across the top carries seven menus -- Room, Tabletop, Fog, Initiative,
  Window, View, Help -- and a vertical icon-only pill floats top-right and
  switches what a click and a drag on the table do: move, measure, fog, draw.
  There is **no side panel**: the table fills everything under the bar. The
  player list is a window opened from the Room menu rather than a column, and
  the initiative tracker is a strip across the top of the table that exists
  only while the tracker has entries -- decided 2026-09-09 in the phase 6
  plan, which is where the reasons are. It is the one live panel that is not a
  window, because it is read by everybody every few seconds for exactly the
  minutes a fight lasts, and players have no menu to open a window from.
- **There is no chat.** It was in the first draft of this document and is not a
  feature of this app. Rolls and their results are the dice tray, in the Window
  menu.
- **Panels that are not the table are floating windows**, decided 2026-09-07
  after phase 3's player list was built as a fixed panel and rejected. Several
  are open at once, dragged and resized and tucked into corners, snapping flush
  to the table's edges and to each other; position and size are remembered per
  window and which windows were open is remembered per room. A window holds a
  `/fragment/` URL and no view code, so any fragment in the app becomes one --
  which is what makes the layer manager, the grid settings, the dice tray and a
  monster's stat block each a fragment and a menu item rather than a feature.
  This is the old client's one interaction worth carrying forward; it is not
  carried forward as code, because there every window held a lit-html template
  and owned its content's lifecycle. See the Windows section in CLAUDE.md.
- **Pawn labels and hp bars** are the grey area. One DOM element per pawn
  lags at a few hundred and makes z-order under fog awkward, so the sprite is
  drawn and hit-tested in canvas, and a single DOM overlay shows the label and
  menu for the hovered or selected pawn only.

Live DOM panels update by **refetch**: a socket event triggers an htmx fetch of
a room fragment that reads the in-memory room. Six clients doing one GET per hp
change is nothing, and it keeps the socket payload pure JSON. Room fragments
need a membership check, so they get an auth wrapper beside the existing
fragment wrapper. Pushing rendered templ HTML over the socket remains
possible later (it rides in the same JSON envelope as a string) if refetch
ever measures as a problem.

DB-backed content stays on its own routes. The room carries ids, and the stat
block, sheet, and journal fragments are opened from the room page with the
content modal exactly as they are elsewhere.

### Renderer

**TypeScript and WebGL2, no framework.** esbuild is already in the repo for
the journal editor; the renderer is one more entry point.

The renderer inherits the phase 7 spec from the map tiling plan, recoverable
with `git show c3c7579^:PLAN.md`:

- Map-pixel space is native at every level. Tile `(z, x, y)` covers the native
  rectangle at `(x * tileSize << z, y * tileSize << z)`. Tokens, grid, fog and
  pointer math never see `z`.
- The parent of `(z, x, y)` is `(z+1, x>>1, y>>1)`; draw it while children load.
- The tile cache is a texture array, LRU by layer, one instanced draw per
  level. An edge tile's UV maximum is `tileW / tileSize`.
- Fetch is `fetch` to `blob` to `createImageBitmap` to `texImage2D`.
- Level selection replaces mipmaps. No `generateMipmap`.
- Fog is drawn after tiles as one map-sized quad blended over them.
- Tiles have no gutter.

Everything is batched: one instanced draw for tiles per level, one for all
pawns, a procedural grid in the fragment shader with zero geometry, one for
the path, one for strokes, one fog quad. The render loop runs on a dirty flag,
not a fixed interval, and only while something is dirty or a drag is in
progress. The canvas is sized for device pixel ratio.

**WebGL2 over WebGPU for the first version.** WebGL2 runs on every device the
players have; WebGPU is still uneven on Linux and older GPUs. The tile cache is
the only part the API touches and both paths share the ImageBitmap-to-array-
texture model, so it sits behind a small interface and a WebGPU backend later
is a contained job.

### Performance rules

The jank in canvas apps comes from specific habits, not from JavaScript
arithmetic. The features discussed so far cost microseconds per frame against
a 6.9 ms budget at 144 Hz. These rules are what keep it that way:

1. **No work in event handlers.** Handlers record pointer state and set a dirty
   flag. One `requestAnimationFrame` loop updates and renders once per frame.
2. **No allocation in the hot path.** Typed arrays for pawn instances, path
   vertices, and stroke points, preallocated and grown by doubling. GC pauses
   are the "gets worse over time" jank, and this rule removes their cause.
3. **No DOM reads or writes in the frame loop.** Layout thrash is why per-pawn
   DOM lags.
4. **No synchronous GPU stalls.** No `readPixels`, no `getError` in
   production, no main-thread image decode. Tiles decode off-thread via
   `createImageBitmap`.
5. **No unbatched draws.** See the list above.

The one honest shared cost is that htmx swaps run on the same main thread. Room
fragments stay small and targeted so a swap during a drag costs well under a
millisecond of layout.

### Escape hatches

Neither is built now. The renderer's boundary must not preclude them.

- **OffscreenCanvas in a worker.** WebGL2 can run entirely in a worker while
  the main thread keeps htmx and the DOM. It costs input latency and
  complexity, so it answers a measured problem. The renderer is a module that
  takes a canvas and an input stream so it can move without a rewrite.
- **A WASM kernel for one algorithm.** If a real algorithm appears (line of
  sight against walls, polygon union for fog), that one function is compiled
  in whatever language and called from TypeScript with typed arrays in and
  out. Integration, not architecture.

### Verification

In the first week of renderer work, build a stress toggle that spawns 500
pawns and 5,000 strokes and drives a synthetic 240 Hz pointer, and profile it
in the Performance panel. Decisions about workers or kernels are then made from
a flame graph rather than from worry.

---

## Table features

These were discussed as the features whose cost had to be understood before
choosing the renderer language. Their designs are recorded here so the
detailed specs start from them.

### Grid snapping

Snapping is a room-wide GM toggle in the table configuration. One function
covers every case, applied per axis: an odd footprint (1, 3) snaps its centre
to a cell centre; an even footprint (2, 4) snaps its centre to a vertex, which
is the 5e rule for large creatures anyway. The GM's "corner" mode is the same
function with a half-cell offset. Rectangular objects snap each axis by its
own parity. In a group move only the anchor snaps; the rest keep their offsets.

The server applies the same snap on receipt of `pawn.move`, so the stored
position is authoritative and a client cannot bypass it.

### Movement path

While a pawn is dragged with snapping on, every client draws a ruler from the
pawn's committed position to the dragged cell: a supercover line walk between
the two cells, a highlight per cell, and a distance label using the table's
feet per cell and diagonal rule (5e default counts diagonals as one; 5-10-5 is
the alternative). The origin cell keeps a ghost of the pawn until release.

The path itself is never on the wire. Other clients receive `pawn.dragging`
with only the hover point and compute the same path from the same two cells.
Because the hover only changes at cell boundaries, both the recomputation and
the network message fire a few times a second regardless of mouse rate.

### Drawing and erasing

Strokes are polylines stored in room state as quantised map-space integers.
Each stroke is a triangle strip in one shared vertex buffer and all strokes
draw in one call; a few thousand strokes is a few hundred thousand triangles,
which is nothing to a GPU. Only the in-progress stroke changes between frames,
and only its dirty range is uploaded.

Erase-by-stroke is a segment distance test against stored points. A pixel
eraser, if wanted, paints strokes into a persistent texture and erases with a
destination-out blend; both are cheap. Undo is removing the last stroke. Input
uses coalesced pointer events so a fast stylus loses no samples. Chunks go out
at roughly ten hertz while drawing so other players see the line form.

Because strokes are stored, any texture is a cache and never the source of
truth, and can be rebuilt at whatever resolution a client's display wants.

### Layers

A room holds an ordered list of layers, bottom to top, each a named slot with
at most one map and its own fog. Floors of a building are the case this is
for, and a scene change is the same mechanism. Only one layer renders at a
time. The **active layer** is room state the GM controls and is what players
see; the GM may additionally **view** another layer locally to prepare it,
with a visible indicator, and spawns land on the layer being viewed. Switching
the active layer crossfades the map on every client; players receive the new
floor's pawns and lose the old floor's, through the same removed and spawned
events that visibility uses. The grid is room-wide on the assumption that
layers share an origin and a scale; a mismatched map misaligns rather than
errors, and the layer manager warns when sizes differ. Lower floors
pre-blurred into an export are the map's own pixels; a transparent exterior
could later let the layer below show through for the GM.

### Fog of war

Rectangles and polygons remain the source of truth, one set per layer: they are tiny to sync and
trivially undoable. Each client rasterises them into a low-resolution mask
texture, which is what makes the single fog quad in the renderer spec cheap.

---

## Rejected

Recorded so they are not re-proposed without new information.

- **Odin compiled to WASM with vendored raylib, rendering via WebGPU.** The
  combination does not exist: raylib has no WebGPU backend (6.0, April 2026,
  shipped a software renderer; the wgpu port is a discussion thread), and its
  web build goes through emscripten and renders OpenGL ES through WebGL. raylib
  also expects to own the canvas, main loop, and document-level input, which
  fights an htmx page. The old client's GPU limit (an 8000 px clamp and one
  texture) was a design limit that tiling already fixed, not a language limit.
  If Odin is ever wanted for its own sake, the viable path is `js_wasm32` with
  `vendor:wgpu` or `vendor:wasm/WebGL`, as a pure renderer behind the same
  boundary. Not planned.
- **A "running" flag on the rooms row.** Stale after any crash or deploy.
- **A generic op-code protocol** (the old INSERT/SET/UNSET/BATCH over
  keypaths). Authority and validation become per-keypath, and the reducer
  becomes a partial-update engine. Typed commands and full-entity events
  instead.
- **Per-field pawn events.** Eight variants that each risk drift. One
  `pawn.updated` carrying the whole pawn.
- **A client-trusted hidden flag.** Server-side audience projection instead.
- **One DOM element per pawn.** Canvas sprites with one overlay for the
  selected pawn.
- **Announcement events.** Toasts derive from state events.
- **MessagePack, CBOR, Protobuf, custom binary.** See Wire format.
- **Per-message compression.** Costs CPU on tiny frames; the snapshot is the
  only frame large enough to benefit and it is sent once.
- **Replay-log catch-up on reconnect.** The snapshot is small; send it.

## Decided since, and still open

The phase plans beside this document settled most of what was open when it
was written. There were six; on 2026-09-09, after phase 5 shipped, the sixth
-- fog, strokes, pings and initiative in one plan -- was split into initiative
(phase 6), fog (phase 7) and strokes with pings (phase 8), so that each ships
as one feature with one verification. On 2026-09-10 phase 8 was split again,
into drawing (phase 8) and pings (phase 9), when the drawing half grew a pen,
three measured shapes, an eraser and a colour picker. In brief, so this
document stays the summary:

- **Snapshot storage**: three columns on the `rooms` row (phase 1).
- **Reaching a room**: `/rooms` lists the GM's persistent, named rooms;
  `/rooms/join` takes a code; the room page is `/rooms/{id}`; joining sets
  the session's room and character (phase 1).
- **What a pawn is labelled with**: a table setting, `none`, `default` or
  `full`, per room. `default` gives the party a word for a monster's health and
  no armour class while the GM reads numbers; `none` takes the label off the
  table for everybody. Player pawns and objects are always exact (phase 2,
  reworked in phase 5).
- **`by` in the envelope**: an optional field, present when a player caused the
  event (phase 2).
- **Limits**: the table of constants in phase 2.
- **Drags and pings of hidden pawns**: a hidden pawn's drag preview is not
  sent to players; pings are visible to everyone (phase 2).
- **DOM controls**: every form and button in the room page posts over HTTP and
  the handler dispatches into the hub; the socket carries what originates on
  the canvas (phase 3).
- **Layers**: per-room ordered layers with one active for players and a GM-only
  viewed layer; pawns, fog and strokes belong to a layer; moving pawns between
  layers is GM only; removing a layer deletes what is on it behind the confirm
  modal; the grid stays room-wide (phases 2, 4, 5, 7 and 8).
- **Where the projection happens**: in `Apply`, not in the hub. `Apply` has
  the state, so an event addressed to players is built holding the player's
  copy of the pawn. Only an event whose audience spans both roles carries two
  versions of itself and answers `ForRole`; there are three, and `ForRole`
  may answer nil, which means "this role is told nothing" (phase 2).
- **How `seq` is assigned**: one counter per audience, decided in phase 3 when
  the hub was built. See Wire format above for why.
- **What a gap means**: a server bug, not a reordering. A WebSocket is one TCP
  connection with no multiplexing, so frames arrive in the order they were
  sent and the browser fires `message` in receive order -- there is no reorder
  buffer on the client and none is needed. The backpressure policy is to close
  a client that falls behind rather than to drop frames for it, and a reconnect
  gets a fresh snapshot, so a live connection cannot legitimately see a gap.
  `seq` is therefore a cheap assertion that catches a fan-out that skipped
  somebody, and the client answers one with `sync.request` (phase 3).
- **The refetch pattern's own ordering hazard**: two socket events fire two
  htmx GETs whose responses can land in either order, which would leave a live
  panel showing the older answer. Every refetching panel carries
  `hx-sync="this:queue last"`, which runs them one at a time and keeps only the
  newest pending one. Not `replace`: that cancels the request in flight, and
  htmx reports every cancellation as a console error -- which the ordinary page
  load produces, since the `load` fetch is still open when the first snapshot
  fires the event. The socket is ordered; a pair of HTTP requests is not
  (phase 3).
- **The socket's URL** is `/socket/room/{id}` rather than `/rooms/{id}/socket`.
  That pattern and `/rooms/join/{code}` both match `/rooms/join/socket` with
  neither more specific, which `http.ServeMux` answers by panicking at
  registration; net/http has no way to settle it. The prefix means what
  `/fragment/` means one level up -- it names a kind of response, and this one
  is not HTML at all (phase 3).

Still open, none blocking:

- **Players receive every layer, not only the active one.** `table.updated`
  carries the whole table to everybody, so a player's browser holds the names
  and map references of floors they are not on, and `fog.added` and
  `stroke.began` go to everybody whichever layer they name. Phase 2
  implemented the snapshot to agree with those events rather than quietly
  disagreeing with them. Closing it means projecting those three events per
  audience, which is the same shape of work as the fog-aware projection
  below. A GM naming a layer "The vault behind the fake wall" is the case
  that would make it worth doing.
- **Server-side fog-aware projection.** Phase 7 conceals pawns under fog on
  the client, which means a player's browser holds pawns it does not show.
  Closing that needs a point-in-polygon pass over pawns on every fog change
  and per-pawn visibility events for players. Deferred hardening.
- **Snapshot schema migration.** Schema is 1; a mismatch starts the room
  fresh. A second schema needs a migration path before it ships.
- **Ephemeral rooms.** Phase 1 makes rooms persistent. If GMs turn out to want
  one-night rooms, close-and-delete is a button, not a redesign.
- **WebGPU backend and a worker.** Both remain escape hatches behind the
  renderer's boundary, to be taken up only on measurement.
