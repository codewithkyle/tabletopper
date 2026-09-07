# Phase 3: transport, the hub, and the debug client

Read `plans/vtt-overview.md`, then `plans/phase-1-rooms.md` (the page and the
membership helper) and `plans/phase-2-protocol-core.md` (the types this phase
carries). This phase puts the protocol core on a socket, gives rooms their
actor goroutines, persists snapshots, and stands up the TypeScript build with a
client that can show you the protocol working. There is no canvas yet.

## End state

- `GET /rooms/{id}/socket` upgrades to a WebSocket for the owner and members.
  The first frame the client receives is `snapshot`. Every later frame carries
  a sequence number.
- The room page loads a bundled TypeScript module that connects, reduces events
  into a client state, reconnects with backoff, resyncs on a gap, and reloads
  when the server version changes.
- In development the page shows a debug panel: connection state, sequence
  number, the player list, the last twenty events, and a box to send a raw
  command. Two browser profiles in one room see each other's pings and see the
  lock toggle as a `room.updated` event.
- The members panel is live: a socket event triggers an htmx refetch.
- Room state survives a server restart. Kill the process with two clients
  connected, start it again, and both reconnect to the same state.
- `make check` runs the TypeScript type check and the reducer golden tests as
  well as the Go tests.

## Decisions

1. **One package for the hub, `server/internal/hub`, importing `internal/room`.**
   The core stays pure and testable; the hub owns goroutines, sockets, the
   database and time.
2. **One goroutine per live room, one inbox channel, no locks on state.** Every
   interaction with a room is a message: a connection joining or leaving, a
   command from a connection, a command from an HTTP handler, a tick, shutdown.
3. **One buffered outbound channel per connection, non-blocking send, close on
   full.** Capacity 256 frames. A client that cannot drain that is not keeping
   up; it is closed with status 1008 and a reason of `slow`, and its reconnect
   gets a fresh snapshot.
4. **HTTP handlers reach the hub through `Dispatch`.** Every DOM control in the
   room page (lock, and in later phases grid settings, map choice, pawn
   dialogs) is an ordinary htmx form posting to a `/rooms/{id}/...` route. The
   handler builds the command, calls `hub.Dispatch`, and maps a `room.Error` to
   `htmx.Error`. The socket carries only what originates on the canvas or
   needs no form. This keeps validation in the core, errors in the alert modal,
   and the client bundle small.
5. **Snapshots are written at most every five seconds while dirty, on unload,
   and on shutdown.** A room with no connections stays loaded for ten minutes
   so a refresh or a flaky connection is instant, then snapshots and unloads.
6. **The server version is the VCS revision from `debug.ReadBuildInfo`**, with
   `-dirty` when modified, falling back to the start time in development. It is
   the `version` in every snapshot and the `?v=` on the bundle URL, so a
   client that reconnects to a newer build reloads and the one-hour static
   cache does not serve it the old bundle.
7. **Resolution happens in the hub, before the actor.** `table.setLayerMap`,
   `pawn.spawn` and `pawn.spawnCharacters` need rows. The hub reads them on the
   connection's goroutine, fills the resolved field, and only then sends the
   command into the room's inbox, so the actor never waits on the database.
8. **TypeScript is type-checked by `tsc` and bundled by esbuild.** esbuild strips
   types without checking them. `typescript` becomes a devDependency and the
   `js` target runs `tsc --noEmit` first, so a type error fails the build the
   way a Go compile error does.
9. **Reducer tests run under `node --test` with Node's type stripping.** Node 24
   runs `.ts` files directly when they use erasable syntax only. The client
   code therefore uses no enums, namespaces or parameter properties, which is
   good TypeScript anyway. No test framework is added.

## Server

### `server/internal/hub`

```
hub.go        Hub: live rooms map, Load, Dispatch, Notify, Close, Shutdown
actor.go      the per-room goroutine and its message types
conn.go       one WebSocket connection: read loop, write loop, send channel
resolve.go    fills resolved commands from the database
persist.go    Store interface, snapshot load and save, the dirty timer
version.go    Version() string
handler.go    the HTTP upgrade handler
```

**`Hub`**

```go
type Hub struct { /* mutex-guarded map[ulid.ULID]*actor, deps */ }

func New(q *queries.Queries, opts Options) *Hub
func (h *Hub) Serve(ctx context.Context, w http.ResponseWriter, r *http.Request, roomID ulid.ULID, actor room.Actor, sess session.UserSession)
func (h *Hub) Dispatch(ctx context.Context, roomID ulid.ULID, actor room.Actor, cmd room.Command) error
func (h *Hub) Notify(roomID ulid.ULID, cmd room.Command)            // hub-only commands; no-op if the room is not live
func (h *Hub) Close(ctx context.Context, roomID ulid.ULID)          // room.close, drop connections, unload without saving a new code
func (h *Hub) Players(roomID ulid.ULID) ([]room.Player, bool)      // for the members fragment
func (h *Hub) Shutdown(ctx context.Context)                         // snapshot every room, close every connection with 1001
```

`Dispatch` is synchronous: it sends the command into the inbox with a reply
channel and waits for the actor's answer, bounded by `ctx`. `Dispatch` loads the
room if it is not live, because a GM changing settings from a page with no
socket open still expects them to persist.

The `Options` carry the tunables so tests set them short: snapshot interval,
unload grace, drag flush interval, send buffer, read limit, command rate.

**The actor**

State owned by the goroutine: `*room.State`, the connections in the room with
their role and user id, `seq`, the dirty flag and last save time, a map of
pending drag positions by pawn id, and a monotonic ULID source
(`ulid.Monotonic(rand.Reader, 0)`) for `room.Env.NewID`.

Inbox messages: `join{conn}`, `leave{conn}`, `command{conn, cmd, cid}`,
`dispatch{actor, cmd, reply}`, `tick`, `shutdown{done}`.

On `command` and `dispatch`: `Authorize`, then `Apply`, then for each emission:
`seq++`, resolve the audience to connections, encode once per role that will
receive it (via `ForRole`), enqueue. An `Error` from either step goes back to
the sender as an `error` event with the `cid` (for a `command`) or as the
return value (for a `dispatch`). Any emission whose event is not transient
marks the room dirty.

Two hub-side effects are triggered by events, not commands: `player.kicked`
closes that user's connections and runs `ClearUserRoomSessions`; `room.closed`
is emitted only by `Close` and is followed by unload.

`pawn.drag` is not applied like the others. The actor stores the latest
command per anchor and `tick`, every 50 ms, emits one `pawn.dragging` per
pending anchor, with the previewed position of every pawn in that drag. Twenty updates a second is smooth and bounds the cost of a
fast mouse.

**Join order.** On `join`, the actor applies `player.join` and emits to the
existing connections, then encodes `snapshot` for the joining connection's role
with `you` and `version`, enqueues it, and only then adds the connection to its
set. The joiner never sees an event with a sequence number at or below its
snapshot's, and the existing clients see the join.

**Leave order.** On `leave`, remove the connection; if the user has no other
connection in the room, apply `player.setConnected{false}`. If no connections
remain, start the unload timer.

**Connection**

The read loop decodes with `room.DecodeCommand`, enforces the rate limit (a
token bucket of 60 per second with a burst of 120; over it, an `error` with
code `rate_limited`; five overs in a minute, close with 1008), and posts to the
inbox. The write loop drains the send channel with a five-second write
deadline per frame. `SetReadLimit(64 << 10)`. Either loop ending closes the
connection and posts `leave`.

**Handler**

```go
mux.HandleFunc("GET /rooms/{id}/socket", auth.RequireSessionOr404(app.RoomSocket))
```

`RoomSocket` runs `loadRoomMember` (a non-member is 404), refuses a closed room
for players, clears the write deadline with `http.ResponseController` as the
upload handlers do, and calls `hub.Serve`. `Serve` accepts with
`websocket.Accept(w, r, nil)`: the library's default origin check requires the
`Origin` host to equal the request host, which is the policy we want, so no
`OriginPatterns` are set. Compression stays off.

**Persistence**

`Store` is an interface with `Load(ctx, roomID) (Loaded, error)`, where `Loaded`
carries `Name`, `Locked` and `Snapshot json.RawMessage`, and
`Save(ctx, roomID, snapshot []byte, seq uint64) error`; the production
implementation wraps two new queries in `server/sql/rooms.sql`:

| Name | Kind | Statement |
| --- | --- | --- |
| `GetRoomSnapshot` | `:one` | `SELECT name, is_locked, snapshot FROM rooms WHERE id = ? AND closed_at IS NULL` |
| `SaveRoomSnapshot` | `:execresult` | `UPDATE rooms SET snapshot = ?, snapshot_seq = ?, snapshot_at = NOW() WHERE id = ?` |

And in `server/sql/session.sql`:

| Name | Kind | Statement |
| --- | --- | --- |
| `ClearUserRoomSessions` | `:execresult` | `UPDATE sessions SET room_id = NULL, character_id = NULL WHERE room_id = ? AND user_id = ?` |

Loading: `room.Unmarshal`; `ErrEmpty` and `ErrSchema` both produce
`room.NewState` with the row's name and lock state, the schema case logged at
warn. On every load, `Room.Name` and `Room.Locked` are overwritten from the
row, which is the writer of record for both.

Saving runs on the actor's goroutine with a two-second context. A failed save
logs and leaves the room dirty so the next tick retries.

**Startup and shutdown** in `main.go`: construct the hub after `tiling.Maps`,
put it on `App` as `Hub`, and call `hub.Shutdown(shutdownCtx)` after
`server.Shutdown` returns. `Shutdown` gives every room the same deadline;
connections are closed with status 1001 so the client reconnects instead of
showing an error.

### Controller changes

- `LockRoom` and `UnlockRoom` call `hub.Notify(id, room.SetLocked{...})` after
  the row is written. `CloseRoom` calls `hub.Close`. `LeaveRoom` calls
  `hub.Notify(id, room.PlayerLeave{...})`. `RenameRoom`, when the header gets
  its edit control, notifies `room.SetName`.
- `RoomMembersFragment` asks `hub.Players` first and falls back to
  `ListRoomMembers` when the room is not live. The rendered list shows the
  connected state.
- `RoomPage` passes `Debug: a.Config.Development()`, `Version: hub.Version()`,
  and the socket path into `RoomPageData`.

### Routes test

`GET /rooms/{id}/socket` resolves to its pattern; the cross-site test gains
nothing (it is a GET), but `TestSameOriginMutationsReachTheSessionCheck` should
confirm the socket route sits behind the 404 wrapper, not the redirect one.

## Client

### Build

- `server/js/tsconfig.json`: `strict`, `noEmit`, `target` and `lib` ES2022 plus
  DOM, `module` ESNext, `moduleResolution` bundler, `isolatedModules`,
  `verbatimModuleSyntax`, `erasableSyntaxOnly`, `include: ["room/**/*.ts"]`.
- `package.json`: add `typescript` pinned to the current major as a
  devDependency. No scripts block; the Makefile invokes binaries directly as
  it does for esbuild.
- Makefile `js` target: prepend `./node_modules/.bin/tsc -p ./server/js/tsconfig.json`
  and add a second esbuild invocation with the same flags plus `--sourcemap`
  for `./server/js/room/main.ts` to `./server/public/static/room.js`. Add a
  `js-test` target running `node --test ./server/js/room/*.test.ts` and put it
  in `check` after `test`. Update the comment above `js` so it no longer says
  Tiptap is the whole reason the repo has a `package.json`.
- `.gitignore` already covers `node_modules`; the built `room.js` is committed
  like `journal-editor.js`.

### Modules under `server/js/room/`

```
main.ts        reads the mount element's data attributes, wires socket, store, panels
protocol.ts    generated by phase 2; never edited by hand
socket.ts      connect, reconnect, gap detection, version check, send
store.ts       State, reduce(state, event), subscribe
reduce.test.ts replays the golden fixtures
panels.ts      dispatches DOM events that make htmx panels refetch
debug.ts       the development panel
```

**Mount contract.** The room page's mount element carries what the client
needs and nothing else:

```html
<div id="tabletop" data-room="01H..." data-socket="/rooms/01H.../socket" data-role="gm"></div>
```

`main.ts` reads these; there are no globals.

**`socket.ts`**

- Connects to `data-socket` with the same-origin `WebSocket`; cookies ride
  along.
- Reconnect on any close except 1008 with reason `kicked` or a `room.closed`
  event, with exponential backoff from 500 ms to 15 s plus jitter, and an
  immediate attempt when the page regains visibility.
- Tracks `seq`. The first `snapshot` sets it. An event with `seq <= current` is
  dropped. An event with `seq > current + 1` triggers one `sync.request` and
  buffers nothing; the snapshot that answers resets everything.
- Remembers `version` from the first snapshot. A later snapshot with a
  different version calls `location.reload()`.
- `send(cmd)` assigns a `cid` (a counter as a string), stringifies, and, when
  disconnected, drops the command and reports it. Nothing is queued while
  offline. The old client's offline queue replayed stale moves after
  reconnect, and every command here is either idempotent by snapshot or
  transient.
- Exposes an `onEvent` subscription. Transient events go to effects; the rest
  go to `store.apply` first, then to subscribers.

**`store.ts`** ports `room.Reduce` from Go line by line: singletons replace,
collection items upsert or delete by id keeping the canonical sort, hot paths
mutate fields, transient events are ignored. `reduce.test.ts` loads every
fixture from `../../internal/room/testdata/reducer/` and asserts deep equality
after each step for both roles. A Go change to the reducer regenerates
fixtures and fails this test until the port follows.

**`panels.ts`** maps event families to DOM events on `window`:
`player.*` to `room:players`, `initiative.updated` to `room:initiative`,
`room.updated` to `room:info`. A panel in templ refetches itself declaratively:

```html
<div id="members" hx-get={ "/fragment/room/members?room=" + data.ID } hx-trigger="room:players from:window" hx-swap="outerHTML">
```

That is the refetch pattern from the overview, and it needs no JavaScript in
the panel.

**`debug.ts`** fills a templ-rendered panel, present only when
`RoomPageData.Debug` is true. It shows connection state, `seq`, `version`, the
player list from the store, the last twenty events as one-line JSON, and a
textarea with a Send button that posts raw JSON through `socket.send`. All
classes are in the templ; the script sets text and toggles `hidden`.

### Room page changes

- `layouts.Base` gets `"/static/room.js?v=" + data.Version` as its script.
- The mount element gains its data attributes.
- The members panel gains its `hx-trigger`.
- The debug panel is rendered when `data.Debug`, inside a `<details>` in the
  side column.

## Tests

**Go, `internal/hub`**, against an in-memory `Store` and a fake connection type
behind a small interface so the actor is tested without sockets:

- Join sends a snapshot first, with `seq` equal to the state's, and existing
  connections receive `player.joined`.
- A command from a player that fails `Authorize` produces one `error` frame to
  that connection with its `cid` and nothing to anyone else.
- A GM `pawn.setVisible` to hidden produces `pawn.updated` on the GM connection
  and `pawn.removed` on player connections, with the same `seq`.
- Three `pawn.drag` commands inside one tick produce one `pawn.dragging` to
  others and none to the sender.
- A full send buffer closes that connection and the room continues.
- Dirty rooms save after the interval and not before; a clean room never saves;
  shutdown saves once and closes with 1001.
- Unload after the grace period saves; a join during the grace cancels it.
- Load of an empty snapshot yields `NewState` with the row's name; a wrong
  schema logs and yields `NewState`.
- `player.kicked` calls the session-clearing statement and closes the target.
- The rate limiter answers `rate_limited` at the boundary and closes after
  repeated overs.

**Go, `internal/hub`, one end-to-end test**: `httptest.NewServer` around the
real handler with the `sessionConnector`-style fake from the middleware tests,
`websocket.Dial` twice with two session cookies, send `ping` from one, read
`pinged` on both, check `seq` ordering.

**TypeScript**: `reduce.test.ts` against the golden fixtures.

## Verification

1. `make js && make check`.
2. Two browser profiles, GM and player, in one room with the debug panel open.
   The player sends `{"type":"ping","x":100,"y":100}`; both panels show
   `pinged` with the same `seq`. The GM toggles the lock; both show
   `room.updated` and the header updates without a reload.
3. Stop the server. Both panels show reconnecting. Start it. Both reconnect
   within seconds, receive a snapshot with the same players, and the debug
   panel shows the sequence continuing from the saved value.
4. Rebuild with a change to `room.js`, restart; both clients reload themselves
   because the snapshot version changed, and the new bundle loads because the
   URL changed.
5. Open the same room in a third tab as the player and close it; the members
   panel still shows the player connected. Close the second tab; it shows them
   disconnected.

## Out of scope

The canvas, any rendering, pawn spawning, resolution against the database for
anything but what `Dispatch` needs (the lock mirror), and the stress toggle.
The `resolve.go` file is created with the interface and the three cases
stubbed to return `invalid`, so phase 4 and phase 5 fill them in.
