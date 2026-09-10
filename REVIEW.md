# VTT review — 2026-09-09

A review of the virtual tabletop as built through phase 6 (rooms, protocol,
hub and socket, renderer, pawns, initiative). Three questions were asked of
every file: can somebody at the table do something they must not, can the
table stall or lose what is on it, and what will phases 7 and 8 have to fight
to land.

This file is a working document. Per CLAUDE.md it must never be referenced
from a code comment, a Makefile or anything that ships, and it is deleted once
the items in it are done.

Scope: `server/internal/hub`, `server/internal/room`, the room controllers in
`server/internal/controllers/room-*.go` and `rooms.go`, the room pages in
`server/templ/pages/room*`, `server/js/room/**`, `server/public/js/room.js`,
`server/main.go`, `server/routes.go` and `server/sql/rooms.sql`. Small things
were deliberately left out; this is the medium-to-large list.

Severity: **Large** is a hole in the security model or a way to lose a
session's work. **Medium** is a real failure that needs an ordinary condition
to trigger, or a structural cost every later phase pays.

## Summary, in the order I would do them

| # | Severity | Finding |
| --- | --- | --- |
| S1 | Large | Membership is checked once, at the upgrade. A kick can be undone by a tab in backoff; leave, logout and delete never reach a live socket. |
| S2 | Large | `DeleteRoom` leaves the live room running. |
| C1 | Large | Graceful shutdown skips the room snapshot on its error path, and shares a deadline that may already be spent. |
| L1 | Large | Refused commands arrive as `error` events and the client discards them. Every server-side refusal is invisible at the table. |
| L2 | Large | No WebGL context-loss handling. A long session ends in a frozen or black canvas with no message. |
| S3 | Medium | No cap on connections per person or per room, and the rate limit is per connection. |
| S4 | Medium | State bounds are per item, not per room. Strokes can grow a snapshot past what the database will store, and the room then never saves again. |
| S5 | Medium | No socket keepalive. A vanished peer stays "connected" indefinitely and keeps the room loaded. |
| L3 | Medium | Failed tile and sprite fetches retry every frame with no backoff, from a loop that is hot for the whole of a fight. |
| C2 | Medium | The snapshot is written on the room goroutine. A slow database stalls the table every five seconds. |
| C3 | Medium | An unreadable snapshot is overwritten by the next save. |
| C4 | Medium | The character sheet write-through races itself, so the sheet can end up holding an older number than the table. |
| L4 | Medium | Relative hit-point entries resolve client-side against a stale base and post an absolute, losing concurrent edits. |
| L5 | Medium | Open pawn windows are neither refetched after a snapshot nor closed for pawns that are gone. |
| L6 | Medium | A group drag keeps the id of a pawn removed mid-drag, and the server then refuses the whole move. |
| L8 | Medium | Initiative refetches are dropped during a drag and a no-op drop never resyncs. |
| L7 | Medium | The selection survives a floor change, so Delete can remove pawns the GM cannot see. (Plausible, not reproduced.) |
| C5 | Medium | `drain` is a hand-maintained exhaustive switch. A view message forgotten there hangs its HTTP caller. |
| S6 | Medium | `resolveMap` runs a database read for any role before authorization. |
| X1 | Medium | Every actor view is three copies of the same plumbing. |
| X2 | Medium | Resolution is a type switch nothing checks. |
| X3 | Medium | The snapshot has no migration path, and phases 7 and 8 will bump the schema. |
| X4 | Medium | The initiative routes are read-modify-write over HTTP and re-implement rules the core already has. |
| X5 | Medium | Seven event names are contracts spelled out in two or three files each. |
| L9 | Medium | The client's event fan-out and tool modes are branch ladders that phases 7 and 8 will extend by adding branches. |
| L10 | Medium | Client copies of server rules (bands, hit-point arithmetic, snapping) are pinned by hand-copied tables; one claimed pin does not exist. |

S = security, C = stability (server), L = client, X = extensibility. Do
S1, S2, C1, L1 and L2 before phase 7 starts; the rest can be interleaved
with it, and X1–X5 and L9–L10 are cheapest done *as* phase 7's first
commits rather than after.

---

## Security

### S1 — Membership is checked once, at the upgrade (Large)

**Where.** `internal/controllers/room-socket.go:47` runs `roomMember` once and
hands `Hub.Serve` a `room.Player`; from then on `client.who` is the authority
for every command (`internal/hub/conn.go:121`, `internal/hub/actor.go:284`).
Nothing re-reads the session or the room row for the life of the socket.
Four things change membership and none of them reaches a live socket except
the kick, and the kick has a race:

- **Kick.** `actor.effect` (`actor.go:609`) closes the kicked user's
  connections and calls `forget`, which clears their session rows in a
  goroutine with a two-second deadline (`actor.go:702`). A tab of theirs that
  was *disconnected* at that instant — in reconnect backoff, floor 500 ms
  (`js/room/socket.ts:36`) — never receives the `kicked` close reason, so it
  reconnects. If its upgrade lands before `ClearUserRoomSessions` commits,
  `roomMember` passes, `join` re-runs `PlayerJoin` and the person is back at
  the table with a fresh socket that no later database write will close.
  Two tabs is the ordinary case for a GM; a player on a flaky connection is
  the ordinary case for this race.
- **Leave.** `LeaveRoom` (`room-page.go:232`) clears *this session's* row
  and notifies `PlayerLeave`, which deletes the player row from state. A
  second session or tab keeps its socket and its `Actor`; `requireControl`
  and `requireOwner` (`room/pawn.go:437`, `:800`) compare against `a.ID`, not
  against `s.Players`, so the person can still move, draw and ping from a
  room whose player list no longer names them, and a reconnect re-seats them.
- **Logout.** `/logout` deletes the session; the socket authenticated by it
  stays open.
- **Delete.** See S2.

**Why it matters.** The kick is the GM's only moderation tool and the design
says so. A moderation action that can be undone by the moderated person's own
browser, without them doing anything, is not one. The other three are
consistency bugs today and become authorization bugs the moment membership
carries more than "may connect" — which phase 7's per-player fog concealment
and any future spectator role will do.

**Fix.**

1. Make the kick's membership clear synchronous and *first*. `KickPlayer` has
   the queries and the request context; run `ClearUserRoomSessions` there,
   before `Dispatch`, and keep `forget` as the belt for the socket-sent path.
   Then the reconnecting tab is refused at `roomMember`.
2. Give the actor a `kicked map[ulid.ULID]time.Time` consulted in `join`:
   a join for an id kicked in the last few minutes is closed with
   `closePolicy`/`reasonKicked`. This closes the window regardless of
   database timing and costs one map lookup per join.
3. Treat `PlayerLeft` the way `PlayerKicked` is treated in `effect`: drop that
   user's remaining connections with a close reason the client treats as
   ended (`left`). A person who pressed Leave in one tab meant it in all of
   them.
4. Re-validate long-lived sockets. Cheapest: `attach` runs a ticker (every
   few minutes) that re-runs the same membership check the upgrade did and
   closes the socket when it fails. This is also what makes logout and S2
   land on live connections without a second mechanism.

**Test.** In `internal/hub`: kick a user who has one connected client and one
that is not; post a `join` for the second before `ClearMembership` resolves
(a fake `Store` that blocks); assert it is closed with `reasonKicked` and the
player row stays gone. In controllers: `LeaveRoom` with two live clients
asserts both close.

### S2 — `DeleteRoom` leaves the live room running (Large)

**Where.** `internal/controllers/rooms.go:156`. The transaction deletes the
row and clears every session's `room_id`; nothing tells the hub. Compare
`CloseRoom` (`room-page.go:137`), which calls `a.Hub.Close` after its
transaction.

**Why it matters.** Everybody connected keeps playing on a room that no
longer exists: commands apply, events broadcast, the actor's `save`
(`actor.go:717`) runs `UPDATE rooms ... WHERE id = ?` against zero rows and
reports success, so `dirty` clears and the log is silent. The room lives until
the last socket drops plus ten minutes. Meanwhile the GM sees it gone from
`/rooms`. Any per-room resource a later phase adds (fog masks in memory,
uploaded sketches) lives on with it.

**Fix.** Call `a.Hub.Close(ctx, roomID)` after the transaction commits, as
`CloseRoom` does — or pull "end the session" into one helper both routes call.
`Close` already emits `room.closed`, drops every connection and saves; the
save will match no row, which is fine. Consider having `Store.Save` log when
`RowsAffected` is zero, so a room saving into a deleted row is at least
visible.

**Test.** Delete a room with a live actor; assert the clients receive
`room.closed`, the hub's `live` is false, and a subsequent `Dispatch` returns
`ErrNoRoom`.

### S3 — No cap on connections per person or per room; the rate limit is per connection (Medium)

**Where.** `internal/hub/conn.go:121` (`attach`) and `:296` (`newBucket`).
Nothing counts connections per user, per room or per process. The token
bucket is constructed inside `readPump`, so it is per socket. There is also no
cap on rooms per owner (`NewRoomForm`, `rooms.go`), and every room page load
by its GM loads an actor.

**Why it matters.** Every join costs the room goroutine a `PlayerJoin` apply,
a broadcast, and a full projected snapshot — `State.Project` is a deep
`Clone` plus a marshal of the whole state (`room/snapshot.go`). A member who
opens N sockets multiplies the command rate by N *and* makes the room encode
N snapshots, on the goroutine that a table full of people is waiting on. A
script (not a browser) has no per-host socket limit. The per-connection bucket
means the "60 a second" ceiling is really "60 a second per socket".

**Fix.** In `attach`, before `join`: refuse (close with `closePolicy`) past a
per-user-per-room cap (4 is generous; a GM with three tabs is the most anybody
has) and a per-room cap (64). Key the bucket by user inside the actor (a
`map[ulid.ULID]*bucket` cleared on the last leave) so the rate is per person,
or keep per-socket buckets and rely on the connection cap — either is fine,
the cap is the important half. Cap open rooms per owner at creation (fifty is
more than anybody runs).

**Test.** Fifth connection for one user is closed with the policy code; the
first four are unaffected. Existing `TestTheRateLimitRefusesAtTheBoundary…`
covers the bucket.

### S4 — State bounds are per item, not per room (Medium)

**Where.** `internal/room/validate.go:54-65`: `StrokesMax` 5 000 ×
`StrokePointsMax` 20 000 is 10⁸ integers a room may legitimately hold; there
is no total. `StrokeExtend.Apply` (`room/stroke.go:141`) checks only its own
chunk and its own stroke. `PlayersCanDraw` defaults to true. `actor.save`
(`actor.go:717`) marshals whatever the state is and retries a failed write
every tick, forever, with `dirty` never clearing.

**Why it matters.** JSON of 10⁸ ints is several hundred megabytes. One
connection at the rate limit (60 frames × 512 points) adds thirty thousand
points a second, so an hour of a player holding a key down — or a script in
minutes with several sockets (S3) — produces a snapshot MySQL refuses
(`max_allowed_packet` defaults to 64 MB). From then on the room cannot be
persisted: every five seconds `save` fails and logs, and a restart or a deploy
rehydrates the room from the last snapshot that fit. Every joiner and every
resync is also sent that state as one frame. This is the one way a *player*
can make a room lose hours of a GM's work.

**Fix.** Add a room-wide point budget (200 000 points is ~1.5 MB of JSON and
more drawing than any session produces) checked in `StrokeBegin` and
`StrokeExtend`, and a per-author share of it for players (a player may hold,
say, a quarter). Give fog the same treatment in phase 7 even though it is GM
only. In `save`, measure the blob; past a soft ceiling log at error level with
the size once, not per tick, and past a hard ceiling refuse to grow the state
(`CodeInvalid: "This room's drawing is full"`) rather than refuse to save it.
Count consecutive save failures and surface the number somewhere a person
will see it.

**Test.** A fixture that fills the budget and asserts the next extend is
refused with the budget's message; a persist test with a `Store` that fails
on size asserts the counter and the single log line.

### S5 — No socket keepalive (Medium)

**Where.** Neither side sends a ping. `coder/websocket` answers pings in
`Read` but nothing here calls `Conn.Ping`; `readPump` (`conn.go:223`) blocks
in `ws.Read(ctx)` with no deadline; the client (`js/room/socket.ts`) only
learns of a dead link when it tries to send.

**Why it matters.** A peer that vanishes without a FIN — laptop lid, NAT
timeout, mobile radio — leaves its `client` in `a.conns` until the kernel's
TCP keepalive fires (hours) or until the room writes to it and the
five-second write deadline trips, which in a quiet room is never. Effects:
the player list shows them connected; `PawnSpawnCharacters` treats them as
present; `idle()` is false so the room never unloads; the slow-client drop
never triggers because nothing is queued. The client side is milder (the
browser usually notices) but a tab that thinks it is connected while NAT has
dropped it shows a table that stopped moving.

**Fix.** In `attach`, run a ticker (30 s) that calls `ws.Ping` with a
ten-second timeout; on failure cancel the socket context so `Read` returns
and `leave` runs. That is the whole of it; the library already answers the
other side's pings. Optionally have the client send a ping frame on the same
cadence so a dead NAT mapping is noticed before the next command.

**Test.** Hub test with a fake conn whose `Ping` errors after N ticks asserts
`leave` is posted; the existing pump tests otherwise cover this path.

### S6 — `resolveMap` runs a database read for any role before authorization (Medium)

**Where.** `internal/hub/resolve.go:71-76`. The two spawn resolvers gate on
`who.GM()` first (`spawn.go:66`, `:303`); `resolveMap` calls
`GetMapPyramid` for whoever sent `table.setLayerMap`, and only then does
`Authorize` refuse the player.

**Why it matters.** A player socket can make the server run a query per
frame at the rate limit, for a command it will never be allowed to apply. It
is small on its own and it is exactly the shape S3 multiplies.

**Fix.** Add the same `if !who.GM() { return nil }` at the top of
`resolveMap`. Structurally, consider giving `Command` a cheap, state-free
`RequiredRole() Role` (or letting `resolve` call `cmd.Authorize(nil-state,
who)` for commands whose Authorize is role-only) so the hub can refuse before
any resolver runs — see X2.

**Test.** A player's `table.setLayerMap` reaches `Authorize` with `Map == nil`
and the fake queries record zero calls.

### Accepted risks, listed so they stay conscious

These are documented decisions, not findings. They are here because phase 7
touches two of them.

- A player is sent a monster's hit points and hides them client-side
  (`room/snapshot.go`, `projectPawn`). Recorded and argued in the code.
- Map tiles and asset images are served to any signed-in user, not only to
  the table (`map-tiles.go`, `imageURL` in `spawn.go`). Previously recorded.
- Players receive every layer's name and map reference, and every layer's fog
  and strokes, in the snapshot (`State.Project`'s comment). **Phase 7 makes
  this a leak rather than a shape:** the fog geometry of the floor the party
  has not reached is the GM's map of what is hidden, and the tiles it hides
  are fetchable. Project the fog and stroke collections by active layer for
  players when the fog work lands, and expect `table.updated` to need a
  player copy too.
- A kicked player can rejoin with the code unless the room is locked
  (`KickPrompt` says so). Fine as long as S1 is fixed; otherwise the lock is
  the only real kick.
- One process, no horizontal scaling (`Hub`'s comment). Fine; the routing
  note is in the right place.

---

## Stability

### C1 — Graceful shutdown skips the room snapshot on its error path and shares a deadline that may already be spent (Large)

**Where.** `server/main.go:138-152`. `server.Shutdown(shutdownCtx)` is given
the whole ten-second budget; on error the function returns before
`rooms.Shutdown` is reached. On success `rooms.Shutdown` is handed the *same*
context with whatever is left of it. `Hub.Shutdown` (`hub.go:439`) posts with
that context; `actor.post` (`hub.go:570`) selects between the inbox send and
`ctx.Done()`, and Go picks between ready cases at random — so with an expired
context roughly half of the rooms are never told to save, and the `<-done`
wait for the rest returns immediately.

**Why it matters.** This is the path the whole snapshot design exists for: a
deploy in the middle of Saturday's game. A long upload holding the HTTP drain
(the upload routes lift their own timeouts) or a stalled database during
`server.Shutdown` means the rooms are not saved, and the next process
rehydrates each from a snapshot up to five seconds — or, with S4, hours —
old.

**Fix.** Give the rooms their own budget and run them on every path:

```go
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    rooms.Shutdown(ctx)
}()
```

placed after the listener stops (the order in the existing comment is right;
`defer` after `server.Shutdown` returns keeps it). Inside `Hub.Shutdown`, post
with a context that cannot already be done — `context.WithoutCancel` plus a
fresh short timeout — so the message always reaches an inbox that has room,
and only the *wait* is bounded by the caller's deadline. Split the budget: 5 s
for HTTP, 5 s for rooms, rather than one 10 s shared.

**Test.** `TestShutdownSavesEveryRoomAndSendsEverybodyAway` exists; add the
case where the context passed in is already cancelled and assert every room
still saved.

### C2 — The snapshot is written on the room goroutine (Medium)

**Where.** `actor.save` (`actor.go:717`) marshals and calls `Store.Save` with
a two-second timeout, from `run`'s tick, on the goroutine that owns the room.
`writeThrough` and `forget` were moved *off* this goroutine for exactly this
reason (their comments say so), so the save is the one remaining database
call the table waits on.

**Why it matters.** During a fight the room is dirty at every tick, so every
five seconds every command and every drag flush waits behind a database
round trip. Normally that is milliseconds; under contention it is up to two
seconds, and it is two seconds for everybody at the table at once. A save
that times out also leaves `dirty` set and tries again five seconds later,
so a slow database is a table that freezes on a schedule.

**Fix.** Marshal on the goroutine (it must read the state) and hand the blob
to a per-actor saver goroutine with a one-slot mailbox: a newer blob replaces
an unsent one, one write is in flight at a time, and the saver reports back
`(seq, err)` on the inbox so `dirty` clears only when the acknowledged seq is
the current one. `closeRoom` and `shutdown` keep their synchronous final save.
Marshal cost is still O(state); if S4's budget lands, that is bounded.

**Test.** A `Store` whose `Save` blocks for a second; assert a `dispatch`
posted during the block is answered in well under that.

### C3 — An unreadable snapshot is overwritten by the next save (Medium)

**Where.** `internal/hub/persist.go:114-123` (`hydrate`). A schema mismatch
or a decode error starts the room fresh and logs; the first dirty tick then
`UPDATE`s the column, and the only copy of the old state is gone.
`room/snapshot.go:25` says a schema bump is expected to cost "the pawns they
had placed".

**Why it matters.** Today the cost is one table's pawns. After phases 7 and 8
it is a session's fog and drawing too, and a schema bump is a deploy that
throws every room's table away with no way back — the exact thing the
"snapshot column" was added to prevent. A decode *bug* (not a bump) does the
same silently.

**Fix.** Before starting fresh, keep the bytes: a `snapshot_failed` column
(or a small `room_snapshot_failures` table with room id, schema found, blob,
timestamp) written from `hydrate`'s two error branches. It costs one column
and makes both a migration and a manual recovery possible. Pair with X3 so
that a bump does not reach this path at all.

**Test.** `TestAnUnreadableSnapshotStartsTheRoomFreshFromTheRow` exists;
extend it to assert the bad blob is preserved after the first save.

### C4 — The character sheet write-through races itself (Medium)

**Where.** `actor.writeThrough` (`actor.go:657`) starts a goroutine per
`pawn.updated` emitted to the GM, with no ordering between them. It also
fires for every `pawn.updated`, not only hit-point changes: a condition tick
on `initiative.next`, a visibility toggle, a layer move.

**Why it matters.** Two hit-point edits fifty milliseconds apart — "23" then
"16", which is one damage entry corrected — are two goroutines that can commit
in either order. The sheet then reads 23 while the table reads 16, and the
comment's "catches up on the next edit" only holds if that next edit also
wins its race. `initiative.next` with several conditioned player pawns is
several `UPDATE`s per keypress.

**Fix.** A per-actor writer goroutine fed by a small mailbox keyed by
character id, latest value wins, one statement in flight at a time — the
drag-coalescing shape, applied to writes. Only enqueue when the pawn's HP
actually changed: `PawnUpdate.Apply` has both old and new pawn in hand (`p`
and `next`), and it is a one-line comparison there rather than "carry the
before-pawn through the event" as the comment feared.

**Test.** Fake queries with a latch; two updates, release in reverse order;
assert the final stored value is the latest.

### C5 — `drain` is a hand-maintained exhaustive switch (Medium)

**Where.** `actor.drain` (`actor.go:879`) answers messages that arrived in
the instant between a room deciding to retire and leaving the map. Each view
message — `roster`, `tableView`, `pawnView`, `initiativeView`, `spawnView` —
needs a case in `handle`, a case in `drain`, a struct, and a `Hub` method
that is fifteen lines of the same `select`. Nothing checks that the four
stay in step.

**Why it matters.** Phase 7 adds `fogView` (the plan already names
`TableView.Fog`). A view added to `handle` and forgotten in `drain` is a
reply channel nobody writes: `Hub.Table`-style callers wait on the *request*
context, so the HTTP handler hangs until the browser gives up on the request,
and the GM sees a window that spins and then errors — only when a room
happened to be retiring, which is the kind of bug that is reported once a
month and never reproduced.

**Fix.** See X1: one generic message replaces the five, `drain` has one case,
and the property is structural. Failing that, a `replier` interface every view
message implements (`gone()` closes or answers its channel) with a test that
posts one of every message type into a retiring actor and asserts every reply
resolves.

---

## Extensibility

### X1 — Every actor view is three copies of the same plumbing (Medium)

**Where.** `hub.go` `Players`/`Table`/`Pawn`/`Initiative`/`spawn` and the
five message types plus their `handle` and `drain` cases in `actor.go`.

**Why it matters.** The pattern is right — ask the goroutine, get a copy —
but it is written out five times and phases 7 and 8 will add at least two
more. Each copy is a place to forget the projection parameter, the `nil`
answer, or the `drain` case (C5).

**Fix.** One message and one helper:

```go
type ask struct {
    fn    func(*actor) any
    reply chan any
}

func view[T any](ctx context.Context, h *Hub, roomID ulid.ULID, load bool, fn func(*actor) *T) (*T, bool)
```

`handle` runs `m.fn(a)` and sends; `drain` sends `nil`; each hub method
becomes a closure over the projection it needs (`func(a *actor) *TableView {
return a.table() }`). The load-versus-no-op distinction stays a parameter,
and the comment explaining which each caller wants stays on the caller.

### X2 — Resolution is a type switch nothing checks (Medium)

**Where.** `hub.resolve` (`resolve.go`) switches on three command types whose
`json:"-"` fields it fills. The contract "a command with a `json:"-"` field
must be resolved here" is enforced only by `Apply` refusing a nil field at
runtime.

**Why it matters.** The next resolved command (a fog import from a map's
dimensions, a stroke with a named brush, a pawn spawned from a shared
monster) is one more case somebody has to know to add, and the failure is a
correct-looking command answered "Map not ready" at a table.

**Fix.** Either a reflection test in `internal/hub` that walks
`room.WireCommandPrototypes()`, finds every struct with a `json:"-"` field,
and asserts `resolve` changes it (or a `Resolved` marker method), or invert
the dependency: `room` declares `type Resolver interface { Map(ctx, who,
asset) (*MapRef, error); Monster(...); ... }` and each resolved command
implements `Resolve(ctx, Resolver) error`. The interface version also fixes
S6 by construction, because each command's `Resolve` can check the role it
already knows it needs.

### X3 — The snapshot has no migration path (Medium)

**Where.** `room/snapshot.go:25-53`. `Schema = 2`; a mismatch is `ErrSchema`
and a fresh room. `State.Normalize` (`state.go:672`) already carries four
field-level repairs for snapshots written before a field existed — that is a
migration layer growing without a name.

**Why it matters.** Phase 7 adds per-layer fog state and phase 8 changes
strokes; one of them will be Schema 3, and the current policy is that every
room's table is discarded on that deploy (C3 is what makes it unrecoverable).
The repairs in `Normalize` show the team already does not accept that cost
for small changes; the big ones deserve the same.

**Fix.** Before the next bump: `Unmarshal` decodes into
`map[string]json.RawMessage`, reads `schema`, and applies
`migrations[n](fields)` from `n` up to `Schema` before decoding into `State`.
Move the four `Normalize` repairs into `migrations[1]`/`[2]` where they
belong, leaving `Normalize` as the canonical-form function it is documented
to be. Keep one golden snapshot per past schema in `testdata` and a test that
each decodes to today's shape.

### X4 — The initiative routes are read-modify-write over HTTP (Medium)

**Where.** `internal/controllers/room-initiative.go`. `OrderInitiative`,
`ActivateInitiative`, `RemoveInitiative` and `AddInitiative` each read
`hub.Initiative(ctx, roomID, room.RoleGM)` (`:321`, `:440`), edit a copy,
and dispatch `initiative.set`. `RemoveInitiative` (`:242`) re-implements
"successor from the old order", which `room.dropped` already does; `withPawn`
(`:378`) re-implements the grouping rule that `InitiativeSync.take` has.

**Why it matters.** The file's own comment accepts the two-tab race. The
larger cost is that every tracker gesture is now two trips into the goroutine
plus a controller-side copy of a rule that lives in the core, and the next
gestures (roll for everyone, delay a turn, hold an action, a note on a line)
each add another. `AddInitiative` also reads the GM's view for any member
before the command refuses them — nothing leaks, but the order is backwards
and it will be copied.

**Fix.** Four small commands beside `InitiativeSync`: `initiative.activate
{entry}`, `initiative.remove {entry}` (calls `dropMembers`/`dropped`),
`initiative.reorder {ids}`, `initiative.add {name | pawn}` (calls the same
`take`). `initiative.set` stays for the editor. The controllers become the
one-line shape the layer routes have (`layerCommand`), the rules stop being
duplicated, and each edit is atomic on the room. This is the pattern the file
already chose for Sync, applied to the rest.

### X5 — Seven event names are contracts spelled out in two or three files each (Medium)

**Where.** `room:view` and `room:blood` (`public/js/room.js:144,166` and
`js/room/render/renderer.ts:314-315`), `alert:pending` (`js/room/exit.ts:21`
and `public/js/alert-modal.js:23`), `settings:change` (`internal/htmx`,
`js/room/main.ts:253`, `public/js/account-name.js:28`), `modal:close`
(`internal/htmx/htmx.go:85`, `public/js/content-modal.js:148`,
`js/room/dialogs.ts:97`), and the panel events `room:players`,
`room:tabletop`, `room:pawn`, `room:initiative` (`js/room/panels.ts:60-71`
against `hx-trigger` strings in five `.templ`/`.go` page files).

**Why it matters.** Each is a string a search finds today, and the comments
say "spelled out in both files" honestly. Phase 7 adds `room:fog` and the
fog options pill's own events; phase 8 adds pings and the draw pill. The
count doubles and the failure mode is silent: a panel that stops refetching.

**Fix.** One source. `public/js` already uses ES modules (`room.js` imports
`toast.js`) and esbuild resolves relative imports, so a
`public/js/room-events.js` exporting the names can be imported by both
bundles today with no build change. For the templ side, a Go const per name
in `templ/pages/room.go` and a test that reads the JS file and asserts every
Go constant appears in it — the same shape as `TestProtocolTypesAreCurrent`.

---

## Client

`server/js/room/**` and `server/public/js/room.js`. Security came up clean:
no `innerHTML` in either bundle, every window and modal URL is checked
against `/fragment/`, every `localStorage` read is guarded and shape-checked,
and nothing the client decides is trusted by the server. What follows is
stability and structure.

### L1 — Refused commands arrive as `error` events and the client discards them (Large)

**Where.** The hub answers every refusal with an `ErrorEvent` to the sender
(`actor.go:refuse`). On the client `socket.ts:193` passes it through as
transient, `store.ts:131` returns, and `main.ts` has no branch for it;
nothing in the bundle reads `heading`, `message` or `code`. Four comments say
otherwise — "an error opens a dialog" in `store.ts` and the generated
`protocol.ts`, "answered with an alert modal" in `handles.ts` and
`dialogs.ts` — so the intent is there and the wire is missing.
`public/js/alert-modal.js:40` already listens for an `alert` event carrying
exactly those two fields.

**Why it matters.** A player drags a pawn the GM just hid (`forbidden`); a
GM drags a group so one member lands past `CoordLimit`; a spawn the server
refuses. In every case the ghost vanishes on release, the pawn sits where it
was, and nobody is told. A GM tries three times and concludes drag is
broken. It also hides every server-side refusal from the people testing the
next two phases.

**Fix.** In `main.ts`'s event handler: on `event.type === "error"` dispatch
`new CustomEvent("alert", { detail: { heading, message } })` on `window`.
Consider treating `not_found` specially (prune the selection and say
nothing), since that one is a race the table produces on its own.

**Test.** A `main.ts`-level test is awkward; a small `effects.test.ts` over
the fan-out (see L9) makes this a one-line assertion.

### L2 — No WebGL context-loss handling (Large)

**Where.** `render/renderer.ts:185-227` creates the context once and every
pass captures `gl`, its programs, VAOs and texture arrays at mount. There is
no `webglcontextlost` or `webglcontextrestored` listener anywhere under
`js/`.

**Why it matters.** A four-hour session on a laptop loses its context: GPU
power switching, a driver reset, Chrome reclaiming a backgrounded tab's GPU
memory, most Android browsers. After that every `gl.*` call is a silent
no-op — the canvas freezes on its last frame or goes black, tile uploads
"succeed" into nothing, and there is no message. Without `preventDefault`
on the lost event the browser does not even try to restore. The only
recovery is a reload the person has to think of, mid-fight.

**Fix.** On the canvas: `webglcontextlost` → `preventDefault()`, stop the
frame loop, show a "restoring the table" notice in the slot
`[data-tabletop-unsupported]` already occupies; `webglcontextrestored` →
rebuild every pass, reset the sprite slots and the tile cache, mark pawns
dirty and the last map/epoch unknown, re-read the clear colour. The passes
already expose `dispose()`; the mount body needs to become a `build()` that
can run twice.

**Test.** `WEBGL_lose_context` in the headless probe recipe already in the
memory notes; assert a frame draws after restore.

### L3 — Failed tile and sprite fetches retry on the next frame with no backoff, and the frame loop is hot for the whole of a fight (Medium, Large under load)

**Where.** `render/tiles.ts:360-389` puts only a 404 in `missing`; a 5xx, a
network error or a decode failure is swallowed in `.catch`, dropped from
`inFlight` in `.finally`, and re-queued by `want()` on the next frame,
`end()` starting up to eight at once. `render/renderer.ts:588` keeps the
loop running while anything glows or pulses, which in combat is always.
Blood sprites (`decals.ts`) go through the same loader.

**Why it matters.** A server restart, a broken tile, or a wifi flap during a
fight becomes a request storm from every client at the table — eight in
flight, refilled at frame rate — until the server is healthy again, which is
the moment it least needs it. A decode failure retries forever.

**Fix.** A per-key `failedUntil` with exponential backoff checked in
`want()`; treat decode failures as permanent (`missing`), since retrying
cannot change bytes.

### L4 — Relative hit-point entries resolve against a stale base and post an absolute (Medium)

**Where.** `js/room/hp.ts:40-60`: the capturing `change` listener rewrites
`field.value` to the resolved number *before htmx reads it* (its stated
purpose), from `field.defaultValue`, the number last rendered. The pawn
window declines refetches while a text field has focus (`typingInPanel`,
`room-pawn.go`), and Enter does not blur. The server's own relative branch in
`evaluateHP` (`room-pawns.go:1377`) therefore never sees a relative entry
from this field.

**Why it matters.** GM's window shows 16; a player takes their own character
to 10 (`pawn.updated` arrives, refetch declined); GM types `-4` Enter →
client posts `12` → server sets 12. The right answer, 6, was available on
the server and the comment above the code says the server is the authority.
Lost updates on the one field that is typed every round.

**Fix.** Post the raw entry and let the server apply it — set the display
value after the request is built (an `htmx:before:request` hook, or carry
the entry in `hx-vals`), or post `entry` plus `base` so the server can apply
the delta to its current value when the base is stale.

**Test.** `hp.test.ts` and `TestTheHitPointBoxTakesASum` both exist; add the
stale-base case to the Go side.

### L5 — Open pawn windows are neither refetched after a snapshot nor reconciled against one (Medium)

**Where.** `panels.ts:71` raises four events on a snapshot and `room:pawn`
is not one of them; a pawn window's trigger listens only for
`room:pawn[detail.id === …]`. `window.ts:748` restores every window that was
open, including ones for pawns that no longer exist.

**Why it matters.** After a reconnect (lid closed for three rounds) every
open pawn window shows pre-disconnect hit points and conditions until that
pawn next changes. A pawn removed while offline leaves its window open for
good: the fragment 404s, `noSwap` keeps the stale markup, and its controls
now alert. Restored windows for dead pawns sit in the error pane and are
re-remembered on every save.

**Fix.** On `snapshot`, raise `room:pawn` for every open `pawn:*` window
and close the ones whose id is not in `state.pawns`; `window.ts` needs to
expose the open ids. The same hook serves phase 7's fog window.

### L6 — A group drag's id list is not pruned on `pawn.removed`, and the server refuses the whole move (Medium)

**Where.** `pawns.ts:1557` prunes `selection` and `hovered` on removal but
not the active gesture's `ids`/`origins`; `commit()` (`:680`, `:707`) sends
`others` from that list; `requireControl` answers `not_found` for the
missing id and `PawnMove.Apply` is all-or-nothing.

**Why it matters.** Six goblins dragged across the room snap back on release
because a second GM, or the owning player, removed one mid-drag — and with
L1 unfixed, silently.

**Fix.** In the removal branch, drop the id from the live gesture (cancel it
if it was the anchor), or filter `others` against `state.pawns` in
`commit()` and the drag sender.

### L7 — The selection survives a floor change, so Delete can remove pawns the GM cannot see (Medium, plausible)

**Where.** `Selection` (`selection.ts`) has no floor and is pruned only on
pawn events and snapshots. On another floor the outlines and `bounds()`
filter by `onFloor` so nothing is drawn and the group panel hides, but
`overlay.ts` still arms the hidden `[data-pawn-remove]` with every id and
the Delete key presses it. The confirm text names nothing.

**Why it matters.** A GM selects four cellar goblins, switches floors to
prepare the next scene, hits Delete believing nothing is selected, confirms
"remove what is selected", and loses them. Not reproduced, but every step is
in the code.

**Fix.** Prune the selection when the viewed floor changes
(`renderer.onSettled` already fires there), or filter `deps.selected()`
through `onFloor` in the overlay. Naming the count in the confirm text is a
cheap second guard.

### L8 — Initiative refetches are dropped during a drag and a no-op drop never resyncs (Medium)

**Where.** The strip's trigger declines `room:initiative` while
`data-dragging` is set (`initiative.ts:159`, `room-initiative.go`); the
comment relies on the drop's POST to bring a fresh strip. `onEnd`
(`initiative.ts:262-267`) returns early when `oldIndex === newIndex`.

**Why it matters.** A GM picks up a card and drops it where it was while a
player ended their turn or a tracked goblin was hit. The strip keeps the
old active line and stale blood until the next tracker event, while the
clock (driven from the store) disagrees with it.

**Fix.** In `onEnd`, after clearing `data-dragging`, dispatch
`room:initiative` when nothing was reordered.

### L9 — Phases 7 and 8 will grow two functions by adding branches (Medium, extensibility)

**Where.** `main.ts:298-391` is a sequential ladder of string tests per
event (`touchesPawns` prefix match, `stroke.cleared`, `snapshot`,
`initiative.updated`, `player.kicked`). `pawns.ts` is 1 700 lines in which
modes are booleans on `TableDeps` (`panning`, `measuring`) tested at the top
of `press()`, `armed` and `measured` live outside the `Gesture` union, and
`render/input.ts` takes exactly one `Tool`.

**Why it matters.** Fog adds `fog.*` invalidation and a rect/polygon tool
with reveal/hide options; strokes add an append-only path for
`stroke.extended` (a hot path where a rebuild is wrong), a 20 Hz draw tool
with begin/extend/end, and pings add timed transient state. Each is another
`if` in the ladder and another `deps.fogging()` in `press()`, which couples
stroke drawing to pawn hit-testing inside one closure.

**Fix.** Make `Tool` the unit of a mode: `tools.ts` reports the chosen tool
id, `main.ts` builds one `Tool` per mode and hands `wireInput` a dispatcher
(Escape and cancel fan out to all). For events, give each subsystem an
`event(e)` the way `follow.event` and `table.preview` already have and call
them from one fixed list, so a new family is a new subscriber rather than a
new branch. This is also where L1 lands.

### L10 — Client ports of server rules are pinned by hand-copied tables, one claimed pin does not exist, and cross-bundle names are unpinned (Medium, extensibility)

**Where.** Only the reducer replays generated fixtures. Bands
(`wounds.ts` vs `snapshot.go:hpBand`), hit-point arithmetic (`hp.ts` vs
`evaluateHP`) and snapping (`path.ts` vs `snap.go`) are twin tables copied by
hand. `selection.ts:27` says "a test pins the two together" for
`SELECTION_MAX`; `selection.test.ts` tests the constant against itself and
the only cross-language pin reads `handles.ts` for `OBJECT_PIXELS_MAX`. The
cross-bundle event names in X5 have no check that the two spellings agree.

**Why it matters.** The reducer pin caught a real bug in phase 3 (the
`null` conditions). The three rule tables have no such net, and phase 7
adds fog geometry and phase 8 adds erase hit-testing — two more pairs.

**Fix.** Extend the Go fixture generator to emit
`testdata/rules/{bands,hp,snap}.json` and replay them in TS; extend the
`OBJECT_PIXELS_MAX` pin to `SELECTION_MAX`; the X5 contract file gets a test
that greps both bundles.

---

## Checked and sound

Things looked at specifically and found right, so a later pass can skip them:

- **Role derivation.** `roomMember` derives GM from `rooms.owner_id` and
  player from `sessions.room_id`; nothing stores a role. Every mutation route
  builds an `Actor` from that and lets `Authorize` decide.
- **Hub-only commands.** `DecodeCommand` consults `wireCommands` only;
  `player.join` and friends cannot arrive off a socket. `DisallowUnknownFields`
  refuses a stale client.
- **Projection on both doors.** `hub.Pawn` and `hub.Initiative` take a role
  and there is no accessor to the unprojected pawn from outside `room`.
  `ProjectedInitiative` filters pawns and entries in one pass. Fragment routes
  answer an empty 404 for a pawn the role may not see.
- **Resolution scoping.** Monster, avatar/token and map lookups are scoped to
  the acting GM's own id; the character lookup is scoped by the seat list the
  room holds. Nothing reads an owner off the wire.
- **Socket origin and CSRF.** `websocket.Accept` with no options keeps the
  library's Origin-equals-Host check; `http.NewCrossOriginProtection` covers
  every non-safe route. The socket rides the session cookie only.
- **Deadlines.** `clearSocketDeadlines` works because `SecurityHeaders` does
  not wrap the `ResponseWriter`; the per-frame write deadline is set by the
  write pump.
- **Sequence numbers.** Per audience, transient frames and snapshots do not
  advance them, and `TestEachAudienceSeesItsOwnSequenceWithNoGaps` pins it.
- **Limits.** Every wire value has a named bound in `validate.go` and the
  messages quote them. (The gap is totals, S4, not items.)
- **Error surfaces.** `NewErrorEvent` and `rejectCommand` never leak an
  internal error string; database failures are 500s with the generic text and
  a log line.
- **Kick flow.** Told first, closed second, announced third, membership
  cleared; the client parks the alert and navigates. (The race is S1.)
- **Join.** Code shape checked before the rate limit, the limit before any
  query; character ownership checked against the roster.
- **Modals and windows** refuse any URL that is not `/fragment/…`; window
  geometry and the open set are guarded `localStorage` reads with shape
  checks; titles are set with `textContent`.
- **Templ escaping** covers every user string that reaches an attribute
  (`hx-confirm`, `aria-label`, `title`); `SafeLayerName` is applied where a
  layer name is printed.
- **Two-tab consistency.** Every table and tracker mutation answers 204 and
  the windows refetch on the event, so no tab is corrected by a reply the
  other did not get.

## Things noticed and left out

Small, or already argued in a comment, and not worth a line above:
`actor.initiative` clones the whole state to hand back the table
(`CloneTable` would do); `Hub.Table` loads a room for a player fetching the
layer name; the drag buffer is keyed by anchor across senders so two people
dragging one pawn overwrite each other's pending frame; `Notify` discards its
post error by design; the container gets a new `Version` per start and
reloads every client on restart (documented, a build-arg away); arming a
placement keeps the frame loop at 60 fps until the GM places or cancels,
because `input.dragging()` counts `tool.active()` and the ghost only moves
on pointer events, which already invalidate.
