# Revision: the room protocol, the hub and the socket

Written 2026-09-11 from an audit of `server/internal/room`, `server/internal/hub`,
`server/internal/events` and `server/js/room/{socket,store,panels,effects}.ts`.
This is the plan for reshaping the MVP's realtime layer into a structure that can
carry several years of features. It is a plan, not a diary: each phase says what
must be true when it is done, which tests prove it, and which of those tests are
written first so that the work turns red into green.

## Verdict

The transport and the actor are sound. The command contract is uniform. The wire
contract is generated. Persistence, sequencing, and hardening are done properly.
None of that changes.

The one structural fault is that **role projection is computed in three places
and every command has to remember all three.** A player's view of the room is a
function of the GM's state, but that function is spread across:

1. Commands that emit pre-projected pairs by hand: `to(ToGM, PawnSpawned{full})`
   beside `to(ToPlayers, PawnSpawned{projected})`, in `addPawn`, `pawnUpdated`,
   `PawnRemove`, `TableClear`, `TableRemoveLayer`.
2. Events that carry a hidden per-role payload in unexported fields, swapped in by
   `ForRole` at send time: `PawnMoved.shown`, `PawnDragging.shown`,
   `InitiativeUpdated.player`.
3. `State.Project(role)`, used only by the snapshot.

The cost is bookkeeping that every command must get right and that grows with
every feature touching what a player may see. Today that bookkeeping is
`shownSet` and `shownTransitions` in four commands, a loop in `TableSetOptions`
that re-sends every monster when the label setting flips, a projected
`InitiativeUpdated` re-sent from `PawnSetVisible`, and tracker recomputation in
`PawnRemove` and `TableRemoveLayer`. The convergence test catches a forgotten
branch after the fact. Nothing makes forgetting impossible.

Everything else in the findings is small and follows from fixing this:

- Event granularity is mixed. Table, initiative and room replace the whole
  aggregate; pawns, fog and strokes are fine-grained.
- The client reducer and the server reducer are the same switch written twice.
  The generator covers the types but not their meaning.
- The resolve step is a type switch in the hub over three concrete commands, and
  two of the resolvers read room state through an ask round trip before dispatch,
  so they decide on state that can change before `Apply` runs.
- Side effects are a type switch in the actor over events, and the hit point
  write-through is keyed on `PawnUpdated` addressed `ToGM`, which couples it to
  the audience encoding.
- Drag coalescing is a type assertion in the inbound path.
- `internal/events` holds DOM custom-event names for htmx, not room events.
- The pawn edit form dispatches up to four commands in sequence, so a refusal
  midway leaves earlier changes applied.
- `TestOnlyAChangedHitPointTotalReachesTheSheet` is flaky: the sheet writer
  coalesces to the latest value per character, so the test's second put can
  overwrite the first before the writer goroutine drains.

## The target shape

Four rules. Every phase moves the code toward them and no phase moves it away.

1. **A command validates and mutates. It does not describe what changed.**
   `Apply` returns an error, plus at most a list of transient signals (a ping, a
   drag preview, a kick, a close). It never constructs a state event and never
   decides who sees what.
2. **Projection lives in one file.** `State.Project(role)` is the only place
   that knows what a role may see. Adding a rule about visibility is a change to
   that function and nowhere else.
3. **State events are derived, not emitted.** After a command, the hub compares
   each role's projection before and after and derives the events that carry the
   difference. A command cannot forget to tell players about a pawn that came
   into view, because no command tells anybody anything.
4. **The hub does transport.** Sequencing, fan-out, coalescing, persistence, side
   effects. It does not know what a pawn is beyond what the room package exports.

The trade is a clone and two projections per state-changing command. At the
room's hard limits (1,000 pawns, 200,000 stroke points, 200,000 fog points) that
is a few megabytes of copying and comparison, under five milliseconds. Typical
rooms are two orders of magnitude smaller. Drags, which are the only high-rate
traffic, are transient and skip all of it.

## How each phase is run

- Tests are written or changed first. `make check` is red at the end of that step
  for the reasons the phase names, and for no other reason. A compile error in a
  test file counts as red.
- Then the code, until `make check` is green.
- The golden reducer fixtures under `server/internal/room/testdata/reducer` are
  regenerated with `go test ./internal/room -update` and the diff is read before
  it is committed. A fixture diff is a wire change and is reviewed as one.
- No comments, as ever.

## Phase 0: housekeeping

Small, independent, and worth doing first so the rest of this document can say
"room event" without ambiguity.

### Tests first

- `server/internal/hub/sheet_test.go`: rewrite
  `TestOnlyAChangedHitPointTotalReachesTheSheet` so it controls the writer the
  way `TestSheetWritesLandOneAtATimeAndTheLatestWins` already does. Gate the
  `WriteHP` function on a `release` channel, put 12, release one, rename the
  pawn, put 9, release, and only then assert `[12 9]`. Red today about one run in
  four; deterministic afterwards.
- `server/internal/events/events_test.go` and its package move to
  `server/internal/uievents`. The test that pins `public/js/events.js` against
  `All` moves with it and its `browserModule` path is updated. Red until the
  package is renamed.

### Then

- Rename the package and every import: `internal/htmx`, `templ/pages/room-*.go`,
  `templ/pages/room-*.templ`. The exported constant names stay the same.
  `public/js/events.js` does not move; the browser side never saw the Go
  package name.

### Done when

`make check` is green and `internal/events` no longer exists.

## Phase 1: derive state events from projections

The foundational change. After this phase the words `ToGM` and `ToPlayers` do
not appear in the room package, no event has an unexported field, and no command
constructs a state event.

### The new shapes

```go
type Command interface {
	Authorize(s *State, a Actor) error
	Apply(s *State, a Actor, env Env) ([]Signal, error)
}

type Signal struct {
	Event  Transient
	To     Audience
	Player ulid.ULID
}

type Transient interface {
	Event
	transient()
}

func Derive(before, after *State, role Role) []Event
```

- `Audience` keeps `ToAll`, `ToSender`, `ToOthers`, `ToPlayer`. `ToGM` and
  `ToPlayers` are deleted.
- `Transient` is a marker interface implemented by `Pinged`, `PawnDragging`,
  `PlayerKicked`, `RoomClosed`, `ErrorEvent`, and `Snapshot`. The type system now
  says what `counts()` and the "state event addressed to part of an audience"
  log line in `actor.emit` said at runtime. Both are deleted.
- `Derive` projects both states for the role and returns the events that move
  the first projection to the second. It emits the vocabulary the client already
  reduces, so `store.ts` does not change shape in this phase:
  - `player.joined` for a new id, `player.updated` for a changed one,
    `player.left` for a gone one.
  - `pawn.spawned`, `pawn.updated`, `pawn.removed` likewise. When every changed
    pawn changed in `x` and `y` only, one `pawn.moved` carries all of them.
  - `fog.added`, `fog.removed`.
  - `stroke.began` for a new id. For an existing id: `stroke.extended` with the
    new points when the old points are a prefix and `done` did not change;
    `stroke.ended` when only `done` flipped; otherwise `stroke.began` again with
    the whole stroke, which the reducer already treats as an upsert.
    `stroke.erased` carries every gone id in one event.
  - `table.updated`, `initiative.updated`, `room.updated` when the aggregate
    differs at all.
  - `fog.cleared` and `stroke.cleared` are deleted from the protocol. They were
    compression for removals and the batched forms cover them.
- `ForRole`, `roleView`, `PawnMoved.shown`, `PawnDragging.shown`,
  `InitiativeUpdated.player`, `shownSet`, `shownTransitions`, `pawnUpdated`,
  `initiativeUpdated`, `tableUpdated`, `projectInitiative` as a standalone
  helper, and the label-flip loop in `TableSetOptions` are all deleted. The
  logic in `projectInitiative` moves inside `Project`, which already calls it.
- `PawnDrag` still needs a per-role view of its preview. That is the one
  transient with a projection rule, and it lives beside `Project` as
  `ProjectSignal(s *State, sig Signal, role Role) Event`, which filters the
  dragged positions to shown pawns for players and returns nil when none are
  shown. No hidden field.
- `exec` in the actor becomes: `before := a.state.Clone()`; authorize; apply on
  `a.state`; on error, `a.state = &before` and return the error; otherwise for
  each role, `Derive(&before, a.state, role)`, encode, fan out; then the signals.
  A refused command therefore leaves the state exactly as it found it, which
  today is only true of commands that validate everything before mutating.
- Sequencing is unchanged: derived events count, transients do not, two counters
  per room.

### Tests first

- `server/internal/room/derive_test.go`, new. Table tests over pairs of states,
  built with the existing `world` fixture, asserting the exact event list per
  role. The cases are the special cases this phase deletes, so each one is a
  regression guard for a branch that used to live in a command:
  - A pawn toggled visible: GM gets `pawn.updated`, players get `pawn.spawned`
    with the projected pawn. Toggled hidden: GM `pawn.updated`, players
    `pawn.removed`.
  - Active layer changed: everybody gets `table.updated`; players also get
    `pawn.removed` for the old floor and `pawn.spawned` for the new one; the GM
    gets nothing more.
  - Label setting flipped from default to full: players get `pawn.updated` for
    each shown monster and NPC and for nothing else; the GM gets only
    `table.updated`.
  - Three pawns moved by a group drag: one `pawn.moved` with three positions for
    the GM, and for players only the shown ones, or nothing.
  - A tracked pawn hidden: players get `initiative.updated` with it dropped; the
    GM gets no `initiative.updated`.
  - A stroke extended, then ended, then extended again after a resync rewrote
    it: `stroke.extended`, `stroke.ended`, `stroke.began`.
  - A layer removed: `fog.removed` per shape, one `stroke.erased`, `pawn.removed`
    per pawn, `initiative.updated` if it changed, `table.updated`, in that order.
  - Two identical states derive nothing, for both roles.
  - A refused command derives nothing, because the state was restored.
  All of these fail to compile until `Derive` exists.
- `server/internal/room/convergence_test.go`: rename
  `TestEmittedEventsConvergeOnTheServersState` to
  `TestDerivedEventsConvergeOnTheServersState`. The recorder's `visit` now
  reduces `Derive(before, after, role)` plus `ProjectSignal` of each signal, and
  compares to `after.Project(role)`. Add a second assertion in the same visit:
  reducing each derived event individually onto a copy changes it, so the
  derivation never emits a no-op. This is what stops a lazy differ that sends
  `table.updated` every time.
- `server/internal/room/fixture_test.go`: `delivered` loses the `ToGM` and
  `ToPlayers` cases and the `ForRole` call; `summary` and `audienceName` shrink.
  `world.apply` returns `[]Signal`.
- `server/internal/room/pawn_test.go`, `layer_test.go`, `initiative_test.go`,
  `projection_test.go`, `snapshot_test.go`: every assertion on emission lists
  becomes an assertion on state, or on `Derive` output where the event list is
  the point. There are about forty such assertions. Each is rewritten before the
  command it tests is touched.
- `server/internal/room/reduce_test.go` and the fixtures: `fog.cleared` and
  `stroke.cleared` cases go. The golden fixtures are regenerated after the code
  lands and the diff is read step by step; the state column must not change on
  any step, only the events column.
- `server/js/room/reduce.test.ts`: remove the two cleared events from the
  coverage list. `store.ts` then fails `ts-check` on the `never` exhaustiveness
  check until its two cases are removed, which is the client's whole change.
- `server/internal/hub/actor_test.go`: `TestHidingAPawnUpdatesTheGMAndRemovesItForPlayers`
  and `TestEachAudienceSeesItsOwnSequenceWithNoGaps` keep their assertions and
  pass on the new path; add `TestARefusedCommandLeavesTheStateAsItWas`, which
  sends a `PawnSpawnCharacters` whose second pawn fails validation and asserts
  that the first was not added.
- `server/internal/hub/pawn_test.go`: `TestWriteThroughOwesOnlyPlayerPawnsWithASheet`
  stays; the hook it tests is re-keyed in Phase 4.

### Then

- `derive.go`, `Signal`, `Transient`, `ProjectSignal`.
- Strip every command down to validation and mutation. `PawnSetVisible` becomes
  four lines. `TableSetOptions` loses its loop. `TableRemoveLayer` loses half its
  body.
- Rewire `actor.exec`, `emit`, `audience`, `recipients` around `[]Signal` and
  `Derive`. `effect` keeps working on the derived events for now.
- Regenerate `protocol.ts`, the reducer fixtures, and `TRANSIENT_EVENTS`.

### Done when

`make check` is green, `grep -rn 'ToGM\|ToPlayers\|ForRole\|shownTransitions'
server/internal` is empty, and the reducer fixture diff shows the same states
with fewer events.

### Deliberately not in this phase

The wire vocabulary beyond the two deleted events. The frame format. The
resolve step. The client beyond two reducer cases.

## Phase 2: one frame per command, one vocabulary per collection

With events derived, the natural output of a command is one list per role. Send
it as one frame. Then the client applies a command atomically, a bulk operation
is one frame rather than hundreds, and htmx panels refetch once per command
rather than once per event.

### The new shapes

```json
{"type": "changes", "seq": 41, "by": "01H...", "events": [ ... ]}
```

- `seq` counts frames per role, not events. Gap detection on the client is
  unchanged. Transient frames and the snapshot stay as they are.
- The vocabulary becomes regular. Every collection has an upsert and a removal
  carrying lists, and the three deltas that are cheaper than an upsert survive:
  - `players.upserted {players}`, `players.removed {ids}`
  - `pawns.upserted {pawns}`, `pawns.removed {ids}`, `pawns.moved {pawns}`
  - `fog.upserted {shapes}`, `fog.removed {ids}`
  - `strokes.upserted {strokes}`, `strokes.removed {ids}`,
    `strokes.extended {id, points}`, `strokes.ended {id}`
  - `table.updated {table}` for the grid, options and active layer, with
    `layers` no longer inside it
  - `layers.updated {layers}`, `initiative.updated`, `room.updated`
- **The layers are an aggregate, not a collection.** They were planned as a
  collection and cannot be one: `TableMoveLayer` changes nothing but their
  order, and an id-keyed upsert and removal carry membership rather than order,
  so a reorder would derive nothing and the convergence test would catch a
  client left holding the old order. `layers.updated` therefore carries the
  whole ordered list and replaces it, exactly as `initiative.updated` carries
  entries whose order is meaningful. There are four aggregates, not three:
  room, table, layers, initiative. The list is bounded at 20 and a layer
  change is rare, so sending all of it costs nothing.
- `Table.Layers` stays where it is in `State`; the wire splits it out by
  embedding the rest of the table in a `TableSettings` the snapshot flattens, so
  the snapshot is unchanged and a grid change no longer resends every map
  reference.
- Both reducers collapse to one generic upsert and remove per collection plus
  the three deltas and the four aggregates. Adding a collection is a `State`
  field, a line in `Derive`'s collection table, and a line in each reducer's
  table. No switch case.
- `announce` in `panels.ts` runs once per frame and dedupes the DOM events it
  raises within the frame.

### Tests first

- `server/internal/room/derive_test.go`: the expected lists change to the new
  vocabulary. Add cases for layers on their own: rename a layer and only
  `layers.updated` is derived, no `table.updated`; move one and the derived
  list carries the new order.
- `server/internal/room/wire_test.go`: add `TestAFrameCarriesEveryDerivedEventOfOneCommand`
  encoding a `Frame` and reading it back.
- `server/internal/room/reduce_test.go`: the generic reducer is tested once per
  collection with an upsert of two, a removal of one, and a removal of an id
  that is not there.
- `server/internal/hub/actor_test.go`: `TestEachAudienceSeesItsOwnSequenceWithNoGaps`
  asserts one frame per command; `TestDragsCoalesceIntoOneFrameForEverybodyElse`
  is unchanged and must keep passing.
- `server/js/room/reduce.test.ts`: the fixture format gains a `frames` column;
  the coverage list becomes the new names. `socket.test.ts`, new, feeds a frame
  with a gap and asserts a resync, feeds a transient and asserts no seq change.
- `server/js/room/panels.test.ts`: a frame carrying three pawns raises
  `room:pawn` three times and `room:initiative` at most once.
- `server/internal/controllers/room-socket_test.go`:
  `TestTwoBrowsersInOneRoomSeeTheSameEvent` reads frames.
- `server/js/room/debug.ts` has no test; the panel shows a frame as one line
  with its event count and is checked by hand.

### Then

- `Frame` type and encoder in `room`, the hub sends one per role per command.
- `Derive` produces the new vocabulary from a collection table.
- Generic `Reduce` in Go and `reduce` in TypeScript.
- `socket.ts` unwraps frames; `main.ts` fans out per event inside the frame;
  `renderer.event`, `follow.event`, `table.preview` receive the same per-event
  calls they do today.
- Regenerate `protocol.ts` and the fixtures.

### Done when

`make check` is green; a `TableClear` on a room with a hundred pawns produces
one frame per role.

## Phase 3: resolve is a contract the command declares

### The new shapes

```go
type Resolver interface {
	Resolve(ctx context.Context, lib Library, s *State) error
}

type Library interface {
	Map(ctx context.Context, asset ulid.ULID) (MapRef, error)
	Monster(ctx context.Context, id ulid.ULID) (MonsterInfo, error)
	Picture(ctx context.Context, id ulid.ULID, kind PictureKind) (PictureInfo, error)
	Character(ctx context.Context, id ulid.ULID) (CharacterInfo, error)
}
```

- `Library` and its info structs live in `room`. The hub implements it over
  sqlc in `hub/library.go`; that file is the only place `queries` rows are read
  for a command.
- **The library is scoped to one owner when it is built, not per call.** The
  methods were planned to take an `owner`, which would have meant every command
  naming the library it reads and `Resolve` needing the actor to name it with.
  `h.library(who.ID)` hands the command a library it cannot point anywhere
  else, so a command has no way to express a read of somebody else's library
  and `Resolve` keeps the signature above. `Character` is unscoped either way:
  a character belongs to the player, not to the GM whose table it is on.
- `TableSetLayerMap`, `PawnSpawn` and `PawnSpawnCharacters` implement
  `Resolver`. The resolution logic in `hub/resolve.go` and `hub/spawn.go` moves
  beside each command in `room`. The `Library` errors carry `*room.Error` with
  the same headings and messages users see today.
- The hub's `resolve` becomes `if r, ok := cmd.(Resolver)`, with `s` a clone
  from an ask, and stays outside the actor. That clone can be stale, so:
- `Apply` re-validates what resolve decided. `PawnSpawnCharacters.Apply` skips
  a character whose seat is gone or whose pawn is already on the table rather
  than duplicating it. `PawnSpawn.Apply` re-checks the seat, which is the one
  thing its resolve decided; it does **not** refuse a character that is already
  on the table, because its resolve never promised otherwise and refusing would
  be a new rule rather than a re-check. Two pawns for one character both write
  hit points back to one sheet, which is worth fixing on its own and is not
  this phase. `TableSetLayerMap.Apply` already re-checks the layer.

### Tests first

- `server/internal/room/resolve_test.go`, new, with a `fakeLibrary`. Every case
  in `hub/spawn_test.go` and `hub/resolve_test.go` moves here as a test against
  the command, not the hub: the monster read from the manual, another GM's
  monster not found, the NPC stat line from the wire, the object sized by its
  picture, the character portrait fallback chain, the party centred on the map.
  The hub tests that remain assert only that the hub calls `Resolve` for a
  `Resolver` and for the GM alone, which is
  `TestEveryCommandWithAResolvedFieldIsResolvedAndOnlyForTheGM` rewritten to
  check the interface rather than a list.
- `server/internal/room/pawn_test.go`: `TestSpawningThePartySkipsASeatThatLeftAfterResolve`
  and `TestSpawningThePartySkipsACharacterAlreadyOnTheTable`, both red today
  because `Apply` trusts the resolved list.
- `server/internal/hub/library_test.go`, new: the sqlc-backed `Library` against
  the test database mapping each row to its info struct, one test per method.

### Then

- Move the code, add the two re-checks, delete `hub/resolve.go` and
  `hub/spawn.go`.

### Done when

No `queries` row is read outside `persist.go` and `library.go`. `hub.go` still
names the type: it holds the handle and takes it in `New`.

## Phase 4: effects are signals, coalescing is a contract

### The new shapes

- `PlayerKick.Apply` returns a `PlayerKicked` signal to the player and mutates
  the roster; the hub's `effect` handles the signal: drop connections, start the
  kick grace, clear membership. `PlayerLeave` derives `players.removed` and
  needs no effect; the hub drops that user's connections when it sees them gone
  from the roster it projected.
- The hit point write-through is keyed on the GM's derived events: a
  `pawns.upserted` whose pawn is a player pawn with a sheet and an HP that
  differs from `lastHP`. No audience, no event type coupling beyond the
  collection.
- `RoomClose` returns a `RoomClosed` signal to all; nothing else changes.
- `actor.effect` becomes two functions: `signals(sigs)` and `changed(events)`,
  each a short switch over a closed set that the type system names.
- Coalescing:

```go
type Coalescer interface {
	CoalesceKey() string
}
```

  `PawnDrag` returns its anchor. `fromClient` checks the interface. The
  drag-specific map, timer and `flushDrags` are renamed for coalescing in
  general and are otherwise unchanged.

### Tests first

- `server/internal/hub/pawn_test.go`: `TestWriteThroughOwesOnlyPlayerPawnsWithASheet`
  is re-expressed against derived events; add
  `TestARenameOfAPlayerPawnWritesNothingToTheSheet` and
  `TestAHitPointChangeFromTheHTTPFormReachesTheSheet` (a `Dispatch`, not a
  socket command, to prove the hook does not depend on who sent it).
- `server/internal/hub/actor_test.go`: `TestAKickTellsThePersonClosesThemAndForgetsTheirMembership`
  unchanged; add `TestALeaveOverHTTPClosesThatPersonsSockets`, red today
  because `PlayerLeft` is handled by event type and will not be after Phase 2.
  Add `TestAnyCoalescerIsCoalescedByItsKey` with a test-only command type that
  implements `Coalescer`.

### Then

- The two switches, the interface, the rename.

### Done when

`grep -n 'case \*room\.' server/internal/hub/actor.go` matches only the
`Signal` cases.

## Phase 5: atomic forms and the controllers

### The new shapes

- `Batch{Commands []Command}` is a hub-only command. `Authorize` authorizes
  each. `Apply` applies each in order and returns the first error; the actor's
  restore-on-error from Phase 1 makes the whole batch atomic and Phase 2 makes
  it one frame.
- `UpdatePawn`, `UpdatePawnHP` and the other handlers that dispatch more than
  once build a `Batch`.
- `rejectCommand` and `refusePawnForm` stay; the batch surfaces the one
  refusal they already know how to render.

### Tests first

- `server/internal/room/batch_test.go`, new: a batch whose second command is
  refused leaves the first unapplied and reports the second's error; a batch by
  a player containing a GM command is refused before anything applies.
- `server/internal/controllers/room-pawns_test.go`:
  `TestAPawnFormWithABadLayerChangesNothing`, red today because the name and
  hit points are applied before the layer is refused.
- `server/internal/hub/actor_test.go`: a batch is one frame per role.

### Then

- `Batch`, the handler rewrites.

### Done when

`grep -c 'Hub.Dispatch' server/internal/controllers/room-pawns.go` is one per
handler.

## Order and dependencies

```
0 ─┐
   ├─ 1 ─ 2 ─┬─ 3
   │         └─ 4 ─ 5
```

Phase 0 is independent. Phase 1 is the prerequisite for everything. Phases 3 and
4 both depend on 2 and not on each other. Phase 5 depends on 4 only because the
batch's frame test assumes Phase 2's frame and its leave test assumes Phase 4's
signal.

Phase 1 is the large one and is the only phase that should be split across more
than one sitting. If it is, the split is by command family: table, then pawns,
then initiative, then fog and strokes, then players, with the convergence test
red until the last family lands and the per-family `derive_test.go` cases green
one family at a time.

## What this plan does not do

- It does not change the snapshot format or add a schema migration. `State` is
  untouched.
- It does not change per-role projection into per-player projection. It makes
  that a later change to `Project`'s signature and to the key the hub derives
  under, with nothing else to touch. That is the point.
- It does not touch the renderer, the tools, or the windows. They keep receiving
  the same per-event calls; only the envelope around those calls changes.
- It does not remove the server-side reducer. It exists so the convergence test
  can prove the derivation, and after Phase 2 it is the same generic function
  the client runs.
