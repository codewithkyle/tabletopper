# Scenes

Written 2026-09-12 from a read of `server/internal/room/{state,table,snapshot,project,derive,sync,resolve,validate}.go`,
`server/internal/hub/{hub,actor,persist,library}.go`,
`server/internal/controllers/{room-table,rooms,room-page}.go`,
`server/js/room/{store,socket}.ts` and `server/templ/pages/room.go`.

Audited 2026-09-12 against those and `server/internal/room/{stroke,pawn,event,command}.go`,
`server/internal/hub/effects.go`, the registry tests in
`server/internal/room/{authorize,convergence,scenario,wire,preview,coalesce}_test.go`,
`server/internal/hub/resolve_test.go`, `server/js/room/{panels,main}.ts`,
`server/js/room/model/revisions.ts`, `sqlc.yaml` and `server/css/app.css`. Every
claim below was checked against the code; the ones that were wrong are gone.

This plan runs before `plans/hex-crawl.md`, which needs phase 1 (the split
*Grid* window) and phase 5 (the wheel, and the export and import lists); the
hex crawl's phases 0, 1, 3, 4 and 5 wait for nothing here.

A scene is a saved tabletop: the grid, the layers and their maps, the fog, the
drawing, and every pawn that is not a person's. A GM saves one, opens another,
and the table becomes it for everyone at the table at once.

Prep happens in a room with nobody in it. The room already **is** a private
tabletop carrying every tool prep needs, so there is no second editor, no second
renderer and no second set of tools to build, and nothing a GM learns while
prepping has to be relearned at the table. This is the decision the rest of the
plan rests on.

## What is already true

**The client already applies a mid-session snapshot, and everything downstream
of it follows.** `store.ts:66` handles a `snapshot` event with
`Object.assign(state, clone(event.state))`; `socket.ts:193` resets `seq` and
clears `resyncing`; `revisions.ts` bumps every slice so the HUD refreshes;
`panels.ts:announce` fires every room UI event (`room:tabletop` among them, which
is what the header's layer name refetches on) and reconciles the pawn windows
against the new pawn list; `follow.ts` and the `decals` stage handle it too. This
is the reconnect path, exercised on every gap. Nothing on the client changes.

**Snapshots already migrate.** `room.Unmarshal` decodes into a
`map[string]json.RawMessage`, runs the chain in `snapshot.go:migrations` from the
`schema` it finds up to `Schema`, and only then decodes into `State`. Anything
stored in that shape is migrated for free forever.

**Layers are already most of a scene, and fail at three things.** A `Layer` is a
map plus its fog, strokes and pawns, and only the active one renders
(`state.go:Shown`). What it cannot do: carry its own grid (`Grid` is on
`TableSettings`, room-wide), exist before a room does, or be used in a second
room. Scenes are not a replacement for layers — a scene *contains* layers, which
stay what they are, the floors of one place.

**The GM-action pattern is settled.** `room-table.go:layerCommand` is the
template: an HTTP route on the resource URL, a `room.Command` built from the
form, `hub.Dispatch`, an `htmx` error on failure. Scene load is one more of
those; scene save is not a command at all (phase 3).

**The library resolves late, for the GM alone, off the actor goroutine.**
`hub.resolve` runs `Resolve(ctx, lib, s)` in the caller's goroutine against a
clone of the room, only when the actor is the GM, with the library scoped to the
GM's user id. `TableSetLayerMap` already re-reads a map through `lib.Map`. Scene
load is the same call in a loop, and its database reads never touch the actor.

**`Snapshot` is role-projected and `ToSender`.** `sync.go:SyncRequest` builds
`s.Project(a.Role)` and signals `ToSender`. It cannot be broadcast as-is; the GM's
projection would reach players. `actor.snapshot(c)` is the per-client path.

**`actor.exec` does four things after `Apply`, and `broadcast` is three of
them.** `broadcast` marks the room dirty, counts the change, advances the
per-role `seq` and encodes the derived frame; then `emit` sends the signals,
`signals` handles the hub-side ones, and `changed` runs the effects. A command
that skips `broadcast` and nothing else is never saved. Phase 2 is exactly which
of those a resync keeps.

**Both command registries are policed.** `scenario_test.go` has to run every
command in `wireCommands` *and* `hubCommands` (`recorder.uncovered`), every hub
command gets `Authorize` called on its bare prototype, `TestReducerFixturesAreCurrent`
writes what the scenario derived to `testdata`, and `js-test` replays it. A new
command is a scenario step and a fixture regen, or `make check` is red.

**A stroke names who drew it.** `Stroke.By` is the drawer's user id and
`requireOwnStroke` gates extend, end and erase on it for players. It is a
reference to a person and rule 5 covers it.

**Tailwind reads `.templ` files and nothing else** (`app.css:248`,
`@source "../templ/**/*.templ"`). A bare English word in a `.templ` file that
happens to be a DaisyUI class — *card*, *badge*, *menu*, *toast* — emits that
component family into `app.css` silently. Prose lives in the `.go` beside the
template, the way `room-maps.go` keeps `roomMapsEmptyHeading`.

## The shape

Five rules. Every phase moves toward them.

1. **A scene body is a whole `State`, not a subset.** The row holds exactly what
   `room.Marshal` writes, with the people-shaped parts emptied. `room.Unmarshal`
   reads it back, which means **every snapshot migration written from now on
   covers every saved scene with no extra code**, and a scene can never drift
   into a second serialization format that needs its own migrations. Import
   takes the *place* out of the decoded state — layers, active layer, grid, fog,
   strokes, pawns — and leaves the rest of the live room alone. The four table
   options (`PawnLabels`, `PlayersCanDraw`, `InitiativeGrouping`, `FogPrefill`)
   are how a GM likes to run a table, not what is on it; they stay with the room,
   and phase 1 draws that line in the UI before anything crosses it.
2. **A scene belongs to a user, not to a room.** It refers to that user's map
   assets and monster rows, so ownership already lines up; the only rule that
   falls out is that a room loads only scenes its GM owns.
3. **Loading replaces and resyncs. It never derives.** A table swap through
   `Derive` is thousands of removes and upserts in one frame, with
   `movedPawns` and `strokeDeltas` churning over every one. Load sends each
   client its own snapshot instead.
4. **Play never silently overwrites prep.** Saving over an existing scene is
   either a deliberate overwrite or an automatic one the scene asked for.
   Nothing else writes to a scene row.
5. **Nothing in a scene refers to a person.** No players, no character pawns, no
   pawn `OwnerID`, no stroke `By`, no initiative, no rolls, no music, no room
   code, name or lock, no `Seq`. A scene loaded in a different room with
   different people at it must be correct, and the only way to be sure of that
   is for it to hold nothing about anybody.

## The hazards worth naming up front

**A map's tile generation is in the scene.** `MapRef` bakes in `Gen`, `Width`,
`Height`, `TileSize` and `MaxZoom`. Re-tiling a map mints a new `Gen`, and
`GetMapTile` answers a dead one with `missingTile` — so a scene saved in March
and opened in June renders an empty grid and nothing anywhere says why. Load
re-resolves every `MapRef` through `lib.Map` rather than trusting the row. This
is phase 4 and it is not optional. Pawns are a different case: a pawn's `Image`
is a URL that fails soft, and the room already holds stale copies of a monster's
name and hit points (there is no `monster.sync`), so a scene is no worse than the
room it came from. Pawns are loaded as saved.

**Saving mid-fight destroys a prepped encounter.** Six goblins at 3 hit points
overwrite six goblins at 11, and the GM finds out the next time they run it.
Phase 5's per-scene *Keep changes* flag is the answer, and until it exists,
saving always creates a new scene.

**Prepping in a room somebody is in broadcasts the prep.** The room is the
editor, so a GM laying out an ambush with a player connected is showing them the
ambush. Lock the room (`rooms.is_locked` already gates `JoinRoomForm`) or prep
before anyone joins. Worth a line of copy in the Scenes window, not a mechanism.

**A scene row is a full state.** It is stored in the room's own format, so its
size is the size of a busy room's snapshot — tens of kilobytes to a few
megabytes, under the 8 MiB `snapshotSoftLimit` the actor already warns at.
`JSON` is fine for that. What must not happen is the live room carrying twenty
of them at once, which is exactly what growing layers into scenes would have
done.

**sqlc types a column by its name.** `sqlc.yaml` hands `*ulid.ULID` to columns
called `id`, `owner_id`, `asset_id` and a named few; anything else that is
`VARBINARY(16)` comes back as `[]byte`. `rooms.scene_id` and `scenes.preview_id`
both need an override entry, or the generated code compiles and the handlers
fill up with conversions.

## How each phase is run

- Tests are written or changed first. `make check` is red at the end of that
  step for the reasons the phase names and for no other reason.
- Then the code, until `make check` is green.
- `make check` is `fmt-check vet test js-test`; the room bundle is `make js`; the
  wire types are `make protocol`, and `TestProtocolTypesAreCurrent` fails if
  they are stale.
- A new command in either registry is a step in `scenario_test.go` and a
  regenerated fixture set: `go test ./internal/room -update`.
- A migration means `db/migrations/` in dbmate's `-- migrate:up` /
  `-- migrate:down` form, a matching edit to `db/schema.sql` (which is what
  sqlc reads), then `make sqlc`.
- No comments, as ever.

---

## Phase 0: the row and the body

No UI, no routes, no hub. The two halves that everything else stands on.

### Tests first

- `server/internal/room/scene_test.go`: a round trip. Build a state with two
  layers, a map on each, fog, strokes by the GM and by a player, a monster pawn,
  an object pawn, a character pawn, a player, initiative entries, a roll and a
  loaded track. `ExportScene` it, `Unmarshal` the bytes, `ImportScene` into a
  *different* state that has its own players, rolls, music and table options,
  and assert: the layers, active layer, grid, fog, strokes, the monster and the
  object pawn arrived; the character pawn did not; the target's players, rolls,
  music and four table options are untouched; the target's initiative is empty;
  the target's own character pawns are gone (their layers are).
- A second test asserting an exported scene carries no player, no pawn with an
  `ownerId` or `characterId`, no stroke with a non-zero `by`, no stroke that is
  not `done`, a zero `seq` and a zero `room`. This is rule 5 and it is the one
  that would otherwise rot quietly.
- A third asserting an exported scene decodes through `room.Unmarshal` — the
  same entry point a room snapshot uses — so the migration chain is proven to
  apply, not assumed to.

### Then

- `server/internal/room/scene.go`:
  - `func (s *State) ExportScene() ([]byte, error)` — clone; zero `Seq` and
    `Room`; empty `Players`, `Initiative`, `Rolls` and `Music`; drop every pawn
    whose `Kind` is `PawnPlayer` and nil `OwnerID` and `CharacterID` on the
    rest; drop every stroke that is not `Done` and zero `By` on the rest; then
    `Marshal`. A zero `By` means only the GM can erase a prep stroke, which is
    what `StrokeErase.Authorize` already does for the GM.
  - `func (s *State) ImportScene(from *State)` — replace `s.Table.Layers`,
    `s.Table.ActiveLayer`, `s.Table.Grid`, `s.Fog`, `s.Strokes` and `s.Pawns`
    from `from`; clear `s.Initiative`; leave `s.Players`, `s.Rolls`, `s.Music`,
    `s.Room` and the four table options alone; `Normalize`. Player pawns do not
    survive: their `LayerID` names a floor that no longer exists.
    `plans/hex-crawl.md` adds `s.Table.Palette`, `s.Tiles` and `s.Notes` to
    both lists as it lands, zeroing `Tile.By` on export the way `Stroke.By` is.
- `db/migrations/20260912xxxxxx_create_scenes_table.sql`:

  ```sql
  -- migrate:up
  CREATE TABLE scenes (
      id           VARBINARY(16) NOT NULL,
      owner_id     VARBINARY(16) NOT NULL,
      name         VARCHAR(128) NOT NULL,
      body         JSON NOT NULL,
      keep_changes TINYINT(1) NOT NULL DEFAULT 0,
      preview_id   VARBINARY(16) NULL,
      created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
      updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
      PRIMARY KEY (id),
      KEY idx_scenes_owner (owner_id, name)
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
  ALTER TABLE rooms ADD COLUMN scene_id VARBINARY(16) NULL AFTER snapshot_failed_at;

  -- migrate:down
  ALTER TABLE rooms DROP COLUMN scene_id;
  DROP TABLE scenes;
  ```

  `preview_id` is the map asset whose existing 256px preview the scene's card
  shows, at `/assets/images/{id}/preview` like every map card. No new image
  pipeline. `rooms.scene_id` is the scene the room currently has open, which is
  what phase 5 saves back to. There are no foreign keys anywhere in this schema
  and none here: a deleted scene leaves a dangling `scene_id`, and phase 5
  treats that as "nothing open".
- `sqlc.yaml`: `rooms.scene_id` and `scenes.preview_id` as `*nullable_ulid`,
  beside `users.avatar_asset_id`, and for the same reason.
- `server/sql/scenes.sql`: `CreateScene`, `ListScenes`, `GetScene`,
  `UpdateSceneBody`, `RenameScene`, `SetSceneKeep`, `DeleteScene`,
  `CountScenes`. Every one scoped by `owner_id` in the statement, not by a check
  in Go. `ListScenes` left-joins `assets` on `preview_id` and returns the id
  only when the map still exists, so a deleted map is a card with no picture
  rather than a broken image.
- `rooms.sql`: `GetRoom` gains `scene_id`; `SetRoomScene` and `ClearRoomScene`,
  scoped by owner.

## Phase 1: the grid is the scene's, the settings are the table's

The one window that mixes both today, split in two, before anything loads a
scene. `TableSetGrid` and `TableSetOptions` are already separate commands; only
the *Grid & settings* window, its form, `POST /rooms/{id}/grid` and `gridForm`
bundle them, and the bundle is what would have made a scene load look like it
had reset the GM's preferences.

| Window | Fields | Command | Belongs to |
| --- | --- | --- | --- |
| Grid | lines, cell size, offsets, colour, snapping, feet per cell, diagonals | `TableSetGrid` | the scene |
| Table settings | pawn labels, how monsters take their turns, players may draw, prefill fog | `TableSetOptions` | the room |

### Tests first

- `templ/pages/room-table_test.go`: `RoomGrid` renders no field that is not on
  `room.Grid`, and `RoomSettings` renders none that is.
  `TestTheGridFormOffersExactlyTheProtocolsChoices` splits the same way, so each
  form is checked against the enums it offers. The rejection-routing and
  no-self-redraw tests run against both forms and both error blocks.
- The Grid form refetches on `room:resync` and on nothing else: a stray
  `pawnLabels` posted to it is ignored, and a `room:tabletop` in its markup
  still fails `TestTheGridFormDoesNotRedrawItselfOnItsOwnSave`.
- `controllers/room-table_test.go`: `SetRoomGrid` dispatches a bare
  `TableSetGrid` from grid fields alone; `SetRoomSettings` is its mirror;
  `TestASavedGridClearsTheMessageTheLastAttemptLeft` loses its option fields.
- The Tabletop menu carries *Grid* and *Table settings* as two windows with two
  ids, and nothing called *Grid & settings*.

### Then

- `room-grid.templ` keeps the grid half. `room-settings.templ` and
  `room-settings.go` take the four options, a `RoomSettingsPanel` error block,
  a `SavePath` of their own, and `PawnLabelChoices` and
  `InitiativeGroupingChoices` move with them.
- `POST /rooms/{id}/settings` beside `POST /rooms/{id}/grid`;
  `GET /fragment/room/settings` beside `GET /fragment/room/grid`. `gridForm`
  returns a `room.Grid` alone and a new `settingsForm` a `room.TableSetOptions`;
  each handler dispatches one command, no `Batch`.
- Window ids: `grid` stays, so the geometry a GM already left it at survives;
  `settings` is new. Menu labels *Grid* and *Table settings*.
- The Grid window has to redraw when a scene lands under it, and cannot listen
  on `room:tabletop`, which its own autosave fires. `uievents` gains
  `Resync = "room:resync"`, `panels.ts:announce` dispatches it on a `snapshot`
  frame and nothing else does, and the Grid form's trigger is
  `load, room:resync from:window`. A load or a reconnect redraws it; a
  keystroke does not.

## Phase 2: resync

A way for a command to say "I replaced the table; hand every client a fresh
snapshot instead of a diff."

### Tests first

- `server/internal/hub/resync_test.go`, using `newTabletop` and a test-only
  command declared in the test file (the way `resolve_test.go` declares
  `countingResolver`) that rewrites the table and is marked `Resyncing`. A GM
  and a player connected; dispatch it; assert with `only` that each socket
  received exactly one `snapshot` frame and no `changes` frame, that the GM's
  carries a hidden pawn and the player's does not, that each carries its own
  role in `you.role`, that the snapshot's `seq` is one past the last frame each
  client saw, and — through `memStore.saved()` after a `settle` — that the room
  was written to the store. A load that sends both a full diff and a snapshot is
  twice the bytes; a load that is never saved is the bug that will not look like
  one until the server restarts.

### Then

- `server/internal/room/command.go`: `type Resyncing interface { Command;
  resyncing() }`. A marker, nothing on the wire, nothing in `eventTypes`, so the
  client never sees a type for it and `TestEveryFrameIsTheChangesFrameOrATransient`
  has nothing new to police.
- `server/internal/hub/actor.go`: in `exec`, after `Apply`, a command that is
  `Resyncing` takes a second branch instead of `broadcast`: mark `dirty`, count
  the change, `advance` both roles as a changes frame would have, and call the
  existing `a.snapshot(c)` for every connection. `emit` and `signals` still run
  for whatever the command returned; `changed` gets no derived list, which is
  right — the effects it watches (sheet write-through on pawn upserts, drops on
  player removal) are about people, and a resyncing command touches none.
- Not a transient, not a signal. An earlier draft had a `Resynced` marker
  event; `emit` runs before `signals` and encodes every signal it is handed, so
  the marker would have reached the wire as a frame the client has no reducer
  for.

## Phase 3: save

### Tests first

- `server/internal/controllers/scenes_test.go`, on the `roomDB` stub: a GM
  posts a save with a name; assert one `INSERT INTO scenes` lands, that its
  body decodes through `room.Unmarshal`, and that it holds no players. A player
  posting the same gets a 403 and no statement runs.
- A save with an empty name, or a name over `RoomNameLimit`, is a 422 with the
  form re-rendered and its error, and no row. A user at `ScenesMax` gets the
  form back saying so.
- `server/templ/pages/pages_test.go`: the Scenes window renders a card per scene
  and an empty state, and the empty-state prose is a constant in
  `templ/pages/scenes.go` — see the Tailwind note above.

### Then

- Save is not a `Command`: it reads the room and writes a row, and the room
  does not change. `hub.Table` and friends already return views; add
  `hub.Export(ctx, roomID) (*ExportView, bool)` through the same `view` helper,
  answering with the exported bytes and the active layer's map asset (nil when
  it has none) for `preview_id`. The clone and marshal run in the actor
  goroutine like `save` does today, and cost the same.
- Routes:

  ```go
  mux.HandleFunc("POST /rooms/{id}/scenes", auth.RequireSession(app.SaveScene))
  mux.HandleFunc("GET /fragment/room/scenes", auth.Fragment(app.RoomScenesFragment))
  mux.HandleFunc("GET /fragment/room/scene/save", auth.Fragment(app.RoomSceneSaveFragment))
  ```

  The save fragment is the content modal's form: a name, `Close` / `Save
  scene`. On success `htmx.CloseModal`, a toast, and a `room:scenes` event the
  window refetches on — `uievents` gains the name and `internal/htmx` gains the
  helper that sends it, since the header merges with the queued toast.
- `ScenesMax = 200` in `internal/room/validate.go` with the other caps;
  `CountScenes` before the insert.
- `server/templ/pages/scenes.templ` and `scenes.go`: a card grid using the same
  `grid-cols-[repeat(auto-fill,minmax(...))]` the asset manager settled on, each
  card showing the preview, the name and the date, and a *Save scene* button in
  the window's header. The fragment's trigger is
  `load, room:tabletop from:window, room:scenes from:window`, so a load in any
  window marks the open scene. The window goes in the Tabletop menu, GM only:

  ```go
  {Label: "Scenes", Window: RoomWindow{
      ID:     "scenes",
      Title:  "Scenes",
      URL:    d.ScenesPath(),
      Width:  380,
      Height: 480,
  }}
  ```

## Phase 4: load

### Tests first

- `server/internal/room/scene_load_test.go` against a fake `Library`: a scene
  whose map asset resolves to a *new* `Gen` loads with the new one, not the
  saved one. A scene whose map asset is gone, or has not finished re-tiling,
  loads with that layer's map cleared and the layer named in `Missing` beside
  the library's own message. A library error that is not a `*room.Error` fails
  the load outright. A scene's monster and object pawns arrive as saved.
- A test that loading clears initiative and removes character pawns, and that
  the players in the room are untouched.
- `wire_test.go`: `TestSceneLoadIsServerSideOnly`, modelled on
  `TestTheCharacterSyncIsServerSideOnly`. A client that could send a whole
  `State` would be a client that can put anything on the table.
- `scenario_test.go`: a `r.hub("open the prepped ambush", &SceneLoad{...})`
  step, so `recorder.uncovered` is satisfied and the fixtures learn what a load
  derives. The convergence test replays derived changes, which a load still
  produces correctly — the hub just does not send them.
- `authorize_test.go` needs nothing: the hub-registry loop already calls
  `Authorize` on the bare prototype, and a nil `Scene` must not make that panic.

### Then

- `room.SceneLoad{Scene *State; Missing []string}` in `hubCommands` as
  `scene.load`, implementing `Resolver`, `Resyncing` and `Command`. `Authorize`
  is `requireGM`. `Resolve` walks `c.Scene.Table.Layers`: every non-nil `Map`
  re-read through `lib.Map(ctx, ref.AssetID)`; a `*room.Error` clears that
  layer's map and records the layer name and the message; any other error is
  returned. Pawns are not touched (see the hazards). `Apply` refuses a nil
  `Scene`, calls `s.ImportScene(c.Scene)`, and returns no signals — the
  `Resyncing` marker is what makes the hub snapshot everyone.
- The controller loads the row **scoped by the session's user id**, decodes it
  with `room.Unmarshal`, writes `rooms.scene_id` *before* dispatching (the
  snapshot fires `room:tabletop` on every client, and the header fragment reads
  the column — a write after the dispatch races it), dispatches, clears
  `scene_id` back if the dispatch fails, and answers with a toast naming what
  was dropped when `Missing` is non-empty. Nothing about a partial load is
  silent.
- `POST /rooms/{id}/scenes/{scene}/open`, behind a `hx-confirm` naming what the
  load discards: every pawn including the party's, the fog, the drawing and the
  turn order.

## Phase 5: continuity

The half that makes a hex crawl work across a campaign: the map remembers what
was revealed on it, session after session, while a prepped encounter does not
remember last week's corpses.

### Tests first

- Opening scene B while scene A is open writes A's current table back to A's row
  first, when A is `keep_changes`.
- The same with A not `keep_changes` leaves A's row byte-identical.
- *Save changes* on the open scene's card writes it back whether or not it is
  `keep_changes`; on any other scene the button is not rendered.
- `CloseRoom` and `DeleteRoom` write the open scene back when it is
  `keep_changes`, and the export happens before the row is closed or deleted
  (`GetRoomSnapshot` only answers for an open room, so after the close query
  the hub cannot load it).
- `ClearTabletop` clears `scene_id` and writes nothing back.
- A room whose `scene_id` names a deleted scene opens B without error and
  without writing anything back.
- `PawnSpawnCharacters` places the party at the active layer's `PartyStart` when
  it has one, and at the map's centre as today when it does not.
- `server/js/room/table-menu.test.ts`: a secondary click over empty ground
  opens the wheel centred on the pointer and remembers the cell under it;
  near an edge it shifts inward and stays whole; the cell stays outlined while
  it is open; Escape, an outside press, a scroll and a swap inside the host
  all close it; *Party starts here* posts that cell's centre to the
  party-start route; a player gets no wheel; nothing in the module writes a
  class name.

### Then

- One controller helper, `saveBack(ctx, roomID, sceneID)`: `hub.Export` then
  `UpdateSceneBody`, in the request goroutine, so the actor is never blocked on
  a database write. Load-another, *Save changes*, `CloseRoom` and `DeleteRoom`
  all call it; nothing else writes a scene body. Between the export and the
  dispatch that follows it a player can still move a pawn; that pawn's move is
  lost from the scene and kept in the room, which is the right way round.
- `rooms.scene_id` is set on load and cleared by `ClearTabletop`. The clear's
  confirm copy gains a sentence: the open scene is closed without saving.
- `SetSceneKeep` behind a toggle on the scene card: **Keep changes** for maps
  the party explores, off for encounters run more than once. Default off — the
  safe one, since the cost of a wrong *on* is silently destroyed prep and the
  cost of a wrong *off* is one visible click to save.
- Two rooms with the same `keep_changes` scene open both write back, and the
  later one wins. Named here so it is not discovered at a table; not solved,
  because one GM running one campaign in two rooms at once is not a thing this
  is for.
- `Layer.PartyStart *Point` with `Point{X, Y int}`, per layer rather than on
  `TableSettings` because a scene has floors and a coordinate without one is
  meaningless. `cloneLayers` copies it; `Derive` already compares layers with
  `reflect.DeepEqual` so nothing there changes; `make protocol` follows. A new
  wire command `table.setPartyStart{Layer, X, Y}` (GM only; a nil `X, Y` pair
  clears it) with its authorize row and scenario step, behind
  `POST /rooms/{id}/layers/{layer}/party-start`.
- `PawnSpawnCharacters.Resolve` reads the active layer's `PartyStart` before
  falling back to the map's centre. A *Place the party* checkbox on the load
  confirm dispatches it after the load, as a second `Dispatch`, and tolerates
  its "Nobody to place" refusal when nobody is seated. A scene still holds no
  player pawns; it holds a coordinate.
- **There is no table context menu yet.** `modes/select.ts:260` opens the pawn
  menu on a secondary click over a pawn and returns `false` over empty ground.
  This phase adds one, and it is a wheel, not a list: a hub of small circle
  buttons centred on the pointer. `GET /fragment/room/table-menu`
  (`internal/controllers/room-table-menu.go`, `room-table-menu.templ` and
  `.go`) renders the hub for the viewer's role into a hidden host in
  `room.templ` that refetches on `room:tabletop`. Each item sits `absolute` at
  the wheel's centre with a negative margin of half its size and is placed by
  one arbitrary-property class carrying daisyUI's flower transform,
  `translate(calc(cos(var(--degree))*var(--radius)), calc(sin(var(--degree))*-1*var(--radius)))`,
  from `--degree` and `--radius` the server writes inline — that is what lets
  the hex crawl drop a ring of twenty tiles around the same hub later.
  `table-menu.ts`, modelled on `pawn-menu.ts`, is called from `select.ts`'s
  `secondary` over empty ground through a `tableMenu(map, screen)` dep wired
  in `modes/table.ts`; it remembers the cell under the map point, outlines it
  through `overlay.cells` while open, shows the host at the pointer, shifts it
  inward where the pointer is nearer an edge than the wheel's radius, and
  closes on a pick, Escape, an outside press, a scroll, or a swap inside the
  host. *Party starts here* is its first hub item, posting the cell's centre
  to the party-start route, with a GM-only glyph in the `floor-marks` stage so
  the GM can see where it is. It is built as a wheel because
  `plans/hex-crawl.md` phase 6 fills a ring around the hub with the scene's
  tile palette and phase 8 adds *Hex note* to the hub; one gesture, one
  surface, built once, here.
- The room header shows the open scene's name, GM and players alike, so "which
  map is this" is never a question during play: a fragment shaped like
  `RoomLayerName` — a span with `hx-trigger="load, room:tabletop from:window"`
  — reading `rooms.scene_id`. That is why phase 4 writes the column before it
  dispatches.

## Phase 6: the rest of the verbs

Rename, duplicate, delete. Duplicate is what a GM reaches for when a prepped
encounter needs a variant, and it is a row copy with a new id.

### Tests first

- Each verb scoped to the owner; another user's scene id is a 404, never a 403,
  because a 403 confirms the row exists.
- Delete of the room's open scene leaves the table exactly as it is and clears
  `scene_id`.
- Duplicate counts against `ScenesMax`.

### Then

- Scenes are the user's, so the verbs live on the scene, not under a room:
  `PATCH /scenes/{scene}/name`, `POST /scenes/{scene}/duplicate`,
  `DELETE /scenes/{scene}`, and phase 5's `POST /scenes/{scene}/keep`. Each
  answers with the card it changed (or nothing, for delete), and the Scenes
  window swaps it in place. `hx-confirm` on delete. Nothing new in
  `internal/room`.

---

## What this deliberately does not do

- **No scene sharing.** The `shares` table exists and this would fit it, but a
  shared scene refers to map assets and monster rows the recipient cannot read,
  and asset sharing is its own plan.
- **No scene editor page.** The room is the editor. A `/scenes` library page
  listing every scene across every room is worth having one day; it is not worth
  having before the feature is used.
- **No per-layer grid.** The room holds one scene at a time and the scene sets
  the grid, so `Grid` stays on `TableSettings` and `room.Schema` does not move.
- **No re-resolving of pawns on load.** A pawn's image is a URL that fails soft
  and its numbers are already a copy; the room does not re-read them either.
- **No scene-body cap of its own.** A body is a room snapshot, and the room's
  8 MiB soft limit is the ceiling it already lives under.
