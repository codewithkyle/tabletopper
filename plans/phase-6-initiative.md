# Phase 6: initiative

Read `plans/vtt-overview.md`, then `plans/phase-2-protocol-core.md` for the
initiative commands, `plans/phase-3-transport.md` for the refetch pattern, and
"Windows, and how live data reaches them" at the end of
`plans/phase-5-pawns.md`, which is the pattern every fragment here follows.
This phase runs the fight: everybody sees the turn order, the GM builds and
advances it, and the player whose turn it is ends it.

This was the last quarter of a plan that also held fog, strokes and pings. That
plan was split on 2026-09-09, after phase 5 shipped, into this one, phase 7
(fog) and phase 8 (strokes and pings), so that each ships as one feature with
one verification. Nothing in it was built before the split.

## Already built

Everything below the socket exists and is tested. This phase adds no command,
no event and no change to the snapshot.

- `internal/room/initiative.go`: `Initiative{Entries, Active, Round}` and
  `InitiativeEntry{ID, PawnID, Name, Initiative}`. Slice order is turn order
  and the number is informational. `initiative.set` replaces the tracker,
  mints ids for entries that arrive without one, refuses a `pawnId` that is
  not on the table and an `active` that is not an entry, and leaves `Round`
  alone. `initiative.next` is the GM's or the active pawn's owner's; it
  wraps, counts rounds, ticks conditions on the two pawns at the seam and
  emits `pawn.updated` for each pawn it changed. `initiative.clear` empties
  it. `InitiativeMax` is 200 and an entry's name goes through
  `checkRequiredName`.
- `initiative.updated` goes ToAll carrying both copies: `projectInitiative`
  drops the entries of hidden pawns from the players', keeps entries for pawns
  on another floor, and clears `active` when the active entry was dropped.
- `pawn.remove`, `table.removeLayer` and `table.clear` drop the entries of the
  pawns they delete through `dropEntriesFor`, which moves `active` on to the
  next combatant rather than to nothing. `pawn.setVisible` re-emits the
  tracker to players when it names the pawn.
- The client reducer applies `initiative.updated`; `panels.ts` raises
  `room:initiative` for it and again on every snapshot.
- The GM's bar has an Initiative menu with two disabled lines, `Sync tracker`
  and `Clear tracker`, which are the old client's names for its two actions.
- Windows (`openWindow`, `window:close`, `window:retitle`, `nextZ`), the
  content modal, `hx-confirm`, `hub.Table` and `hub.Pawn`,
  `room.ProjectedPawn`, and in the controllers `gmTable`, `pawnActor`,
  `rejectCommand`, `layerCommand` and `PanelFormErrors`; in
  `templ/pages/room-pawn.go` the `typingInPanel` trigger filter,
  `PawnBandText` and `pawnPortrait`.

What does not exist: any markup for the tracker, any route, `hub.Initiative`,
and the client's turn timer.

## End state

- While the tracker has entries, everybody sees a strip across the top of the
  table: one entry per line of the tracker with portrait, name and number, the
  round counter, and the active entry highlighted with its pawn's condition
  chips under it. An empty tracker is no strip at all.
- The GM's Initiative menu opens the Tracker window: the entries as editable
  rows -- number, Up, Down, Remove -- with Add pawns, Sort, Next turn and
  Clear under them. Add pawns is a dialog listing every creature on the table
  not yet in the order with a number beside each, and a row for a named entry
  such as a lair action. The menu also carries Next turn and Clear tracker.
- The GM clicks an entry in the strip to make it the active one.
- The player whose pawn is active sees, in that entry, a count-up timer in
  `MM:SS` that turns warning at one minute and danger at two, and an End turn
  button. Nobody else sees either.
- Conditions with durations tick down as turns pass, and the chips in the
  strip and the rings on the table agree because both read the pawn.

## Decisions

1. **The tracker is a strip over the table, and it is the one live panel that
   is not a window.** The overview left how the tracker is presented to this
   phase. The rule that panels which are not the table are floating windows
   was made about the player list: a column that took a fifth of the table for
   something read once an hour. The tracker is read by everybody, every few
   seconds, for exactly the minutes a fight lasts, and a window would have to
   be opened by each person at the table from a menu the players do not have.
   So it is a fragment positioned over the table's top edge, one entry tall,
   that renders itself `hidden` when the tracker is empty. Outside a fight the
   room has no more chrome than it has today.
2. **The strip is a refetching fragment and nothing in it is filled by the
   client.** The placeholder `room.templ` renders carries
   `hx-trigger="load, room:initiative from:window"` so the first paint fetches
   it; the fragment's own root carries `room:initiative from:window` alone,
   because a root that also said `load` would fetch itself again on every
   swap. Both carry `hx-sync="this:queue last"`, exactly as the player list
   does, and the server renders the strip for the requester: their role
   decides whether entries are buttons, and their user id decides whether the
   active entry carries a timer and an End turn. The one thing the client
   writes into it is the timer's digits, because when a turn began is not
   room state.
3. **Every control is HTTP, End turn included.** The split plan sent End turn
   over the socket so that it "felt instant". One POST is one round trip,
   which is what a socket frame is too, and HTTP gives the button the alert
   modal for a refusal without a line of client code. So `initiative.ts` is
   the timer and nothing else.
4. **Editing is a window and adding is a modal.** The GM keeps the tracker's
   editor open through a fight and adjusts it between turns, which is the
   window's case; choosing which pawns to add is a task with an end, which is
   the modal's. It is the phase 5 arrangement for the pawn panel and its
   rename dialog.
5. **`initiative.set` is the only editing command, so every editing route
   reads the live tracker, changes one thing and dispatches the whole.** Move,
   remove, renumber and activate are each one route in the layer manager's
   shape rather than four new commands: phase 2 made set the one editing
   command on purpose, a `setActive` beside it would be a second way to say
   the same thing, and a new command is a protocol change with fixtures behind
   it. The read and the dispatch are two trips into the actor, and a second GM
   tab can slip between them; the loser's edit is overwritten by the winner's,
   which is the race two tabs already have on the layer manager and is
   harmless.
6. **Adding lists creature pawns on every floor.** The dialog is the GM's, so
   nothing leaks, and a fight that spills down a staircase has combatants on
   two floors. Objects have no turn and are not offered. In a room with more
   than one floor each row says which one, as the pawn panel does, and a
   Select all toggle is what the old client's Sync tracker becomes.
7. **Sort is descending by number, stable, and keeps `active` on the same
   entry by id.** Two 14s stay in the order the GM put them in.
8. **Removing the active entry from the editor moves the turn to the next one
   in the old order**, wrapping, or to nothing when it was the last. It is the
   rule `dropEntriesFor` applies when the pawn itself is removed, written out
   once more in the handler because here the entry is going and the pawn is
   staying.
9. **A pawn's entry is drawn from the pawn, not from the entry's name.** The
   entry carries a name because a free-text line has nothing else, but a pawn
   renamed mid-fight should read the same in the strip as on the table. So the
   strip prints the pawn's current name and portrait, its health as a word or
   as numbers under the room's `pawnLabels` rule through `ExactHP`, and its
   conditions, and falls back to the entry's own name only when the pawn is
   gone.
10. **The timer starts when this browser sees the active entry change**, which
    is the honest reading of the one clock the client has. A reload starts it
    from zero: a turn that began before the tab was opened is a turn whose
    length this tab did not watch. Its tones are plain, warning from one
    minute and danger from two -- two steps rather than the old client's
    three, because a "safe" green for the first thirty seconds was a colour
    that answered a question nobody was asking.
11. **The timer runs on `setInterval`, once a second, only while its element
    exists.** It writes text into the DOM and never touches the canvas, so the
    frame loop has no reason to know about it, and it stops itself on the
    first tick that finds no element, which is what a refetch after the turn
    moves on leaves behind.
12. **`hub.Initiative` answers the projected tracker and the projected pawns
    it names in one message.** The strip needs a portrait and a health word
    per entry, the projection has to be the one the socket applies, and asking
    for each pawn separately would be one trip into the actor per entry. It
    takes a role, like `hub.Pawn`, and for the same reason: a fragment is a
    second door into the state the socket projects on the way out.

## Server

### `hub.Initiative`

```go
type InitiativeView struct {
    Initiative room.Initiative // projected for the role
    Pawns      []room.Pawn     // every pawn this role may see, projected
    Table      room.Table      // for pawnLabels and the floor names
}

func (h *Hub) Initiative(ctx context.Context, roomID ulid.ULID, role room.Role) (*InitiativeView, bool)
```

A fourth read accessor beside `Players`, `Table` and `Pawn`, answered on the
actor's goroutine through an `initiativeView{role, reply}` message. It LOADS
the room, as `Table` does: the strip is fetched on every page load and the
socket connects a moment later anyway. `internal/room` gains the exported half
it needs, `func (s *State) ProjectedInitiative(role Role) Initiative`, which is
`projectInitiative` for a player and `cloneInitiative` for the GM; `Pawns` is
`Project(role).Pawns`, so the GM's list is every pawn and a player's is the
shown ones already through `projectPawn`. `drain` answers the new message with
nil like the others.

### Routes

| Pattern | Wrapper | Handler | Behaviour |
| --- | --- | --- | --- |
| `GET /fragment/room/initiative?room={id}` | `Fragment` | `RoomInitiativeFragment` | Any member. The strip, from `hub.Initiative` for the requester's role, rendered `hidden` when there are no entries. |
| `GET /fragment/room/initiative/editor?room={id}` | `Fragment` | `RoomInitiativeEditorFragment` | GM only, else an empty 404. The Tracker window. |
| `GET /fragment/room/initiative/add?room={id}` | `Fragment` | `RoomInitiativeAddFragment` | GM only. The Add pawns dialog, opened at `lg`. |
| `POST /rooms/{id}/initiative` | `RequireSession` | `AddInitiative` | GM only. Reads the checked pawn ids with their numbers and the optional named row, appends them to the live entries in the order the form listed them, dispatches `initiative.set` with `active` unchanged. Nothing chosen is a 422 in the dialog's error slot. On success `htmx.CloseModal` and 204. |
| `POST /rooms/{id}/initiative/next` | `RequireSession` | `NextInitiative` | GM or the active pawn's owner; the core decides. Dispatches `initiative.next`. 204. |
| `POST /rooms/{id}/initiative/sort` | `RequireSession` | `SortInitiative` | GM only. Decision 7. 204. |
| `DELETE /rooms/{id}/initiative` | `RequireSession` | `ClearInitiative` | GM only, behind `hx-confirm`. Dispatches `initiative.clear`. 204. |
| `POST /rooms/{id}/initiative/{entry}/activate` | `RequireSession` | `ActivateInitiative` | GM only. `initiative.set` with the same entries and `active` set to the entry. 204. |
| `POST /rooms/{id}/initiative/{entry}/move` | `RequireSession` | `MoveInitiative` | GM only. Reads `index`, moves the entry there, dispatches set. A position outside the list is a 422. 204. |
| `PATCH /rooms/{id}/initiative/{entry}` | `RequireSession` | `RenumberInitiative` | GM only. Reads `initiative`, an integer; sets the entry's number, dispatches set. Anything else is a 422 in the editor's error slot. 204. |
| `DELETE /rooms/{id}/initiative/{entry}` | `RequireSession` | `RemoveInitiative` | GM only. Decision 8. 204. |

The per-entry and per-tracker mutations share one helper,
`initiativeCommand(w, r, action, build func(view *hub.InitiativeView, entry ulid.ULID) (room.Command, bool))`,
which is `layerCommand` with a tracker read in front of the build: it
establishes the actor with `pawnActor`, reads the live tracker through
`hub.Initiative` as the GM, parses the entry from the path when the pattern
carries one, refuses an entry that is no longer in the tracker with
`htmx.NotFound`, and turns a refusal into the alert modal with
`rejectCommand`. Every mutation answers 204 and redraws nothing: each ends in
`initiative.updated`, which reaches every client, and the strip and the editor
refetch from that.

`initiative.set` validates every `pawnId` against the table, so an entry for a
pawn removed between the read and the dispatch is a `not_found` from the core
and an alert, never a corrupt tracker.

### Templates

- `pages/room-initiative.templ`, `.go`: three components and their data.
  - `RoomInitiative(data RoomInitiativeData)`: the strip. A `<section
    id="room-initiative">` with the refetch attributes of decision 2,
    `hidden` when `Empty`, positioned over the table's top edge and centred,
    one entry per line on `surfacePanel`, scrolling sideways past a dozen.
    Each entry: the portrait (`pawnPortrait` takes a `RoomPawn`, so the entry
    builds the two fields it reads), name, number, and the GM's Hidden badge
    for a hidden pawn. The active entry is highlighted and carries its
    condition chips in the eight colours the pawn panel already prints, and,
    when `Mine`, a `<span data-turn-timer>` and an End turn button with
    `hx-post` to the next route. For the GM every entry is a `<button>`
    posting to its activate route, and the strip ends with Next turn and the
    round.
  - `RoomInitiativeEditor(data RoomInitiativeData)`: the window. A row per
    entry with a number input (`hx-patch` on `change`, `hx-sync="this:queue
    last"`), Up and Down posting to the move route with the index, and Remove
    with `hx-delete`; then Add pawns (`data-modal-open`), Sort, Next turn, and
    Clear behind `hx-confirm` in the error colour. Its `hx-trigger` is
    `room:initiative from:window` filtered with the `typingInPanel`
    expression the pawn panel uses, so a number being typed is not swapped out
    from under the caret. An error slot at the top through
    `PanelFormErrors`.
  - `RoomInitiativeAdd(data RoomInitiativeAddData)`: the dialog. A checklist
    of candidates with a number input each and the floor name in a multi-floor
    room, a Select all toggle, a named-entry row with a name and a number,
    then Close and Add; it posts to the add route with the 422 trio aimed at
    its own error slot.
- `RoomInitiativeData` carries `RoomID`, `IsGM`, `Round`, `Empty`, the paths
  the markup posts to, and `Entries []RoomInitiativeEntry` with `ID`, `Name`,
  `Number`, `Image`, `Active`, `Hidden`, `Mine`, `HP` (the text under the
  `ExactHP` rule, or empty), `Conditions []RoomPawnCondition` and `Floor`.
  `RoomInitiativeAddData` carries `RoomID`, `MultiFloor`, `Errors` and
  `Candidates []RoomInitiativeCandidate` with `ID`, `Name`, `Image` and
  `Floor`. The controller composes every string; the templates print them.
- `pages/room.go`: the Initiative menu becomes `Tracker` (a `RoomWindow` with
  id `initiative`, 320 by 360), `Next turn` (`Post`) and `Clear tracker`
  (`Post`, `Danger`, with a confirm). `RoomPageData` gains the four paths.
  `room.templ` mounts the strip's element inside `#tabletop` with its `load`
  trigger, so the first render fetches it.
- Words that must not appear in the new markup, as attribute names, values or
  ids, because Tailwind reads a `.templ` file as text and each is a DaisyUI
  component: `card`, `countdown`, `timeline`, `steps`, `indicator`, `status`,
  `list`. An entry is an `entry`, the clock is a `timer`. Run the selector
  diff in CLAUDE.md after every change under `server/templ`.

## Client

### `initiative.ts`

```
initiative.ts   the turn timer: when the active entry changed, and the digits
```

`mountTurns(state)` returns `{ changed(): void; stop(): void }`. `main.ts`
calls `changed()` after reducing a `snapshot` or an `initiative.updated`. It
compares `state.initiative.active` with the value it last saw: a different one
records `since = performance.now()`, the same one leaves it alone, so an entry
renumbered mid-turn does not restart the clock. A tick runs once a second while
`[data-turn-timer]` is in the document; it writes `MM:SS` as `textContent` and
sets `data-turn-tone` to `warning` at sixty seconds and `danger` at a hundred
and twenty. `changed()` starts the interval when there is an active entry, and
the first tick that finds no element stops it. No class name is written in
this file; `room-initiative.templ` styles the two tones off the attribute.

Nothing else on the client changes. The strip and the editor are htmx.

## Tests

**Go**:

- `hub.Initiative` projects: a player asking for a tracker that names a hidden
  pawn gets no entry for it and a nil `active` when that was the active one,
  and the pawns it answers with carry a band and no armour class on the
  default setting. This is the security test and it is written first.
- `SortInitiative` orders descending, keeps ties in their existing order, and
  preserves `active` by id.
- `RemoveInitiative` of the active entry activates the next in the old order,
  wraps from the last to the first, and clears `active` when the tracker
  empties -- and `initiative.set` is dispatched exactly once.
- `MoveInitiative` past the end is a 422; `RenumberInitiative` with text is a
  422 in the editor's error slot.
- `AddInitiative` refuses an object's id and an empty submission, appends in
  the order the form listed, copies each pawn's name into its entry, and
  closes the modal.
- `NextInitiative` from a player who does not own the active pawn is the
  core's `forbidden`, rendered as a 403 alert.
- Template tests pin: the strip's `hx-trigger` and `hx-sync`, and that the
  fragment's root does not carry `load`; `hidden` when empty; End turn and the timer only on the active entry and only for its
  owner; entries as buttons only for the GM; the Hidden badge only for the
  GM; the editor's trigger filter carrying `typingInPanel`.

**TypeScript**:

- `initiative.test.ts`: `changed()` resets `since` only when `active` differs
  from the last value seen; the tone is plain at fifty-nine seconds, warning
  at sixty, danger at a hundred and twenty; no interval starts without an
  active entry.
- `panels.test.ts` already asserts `initiative.updated` raises
  `room:initiative` and gains nothing.

## Verification

1. `make js && make check` and the CSS selector diff, which should show only
   selectors the strip, the editor and the dialog use.
2. GM opens Tracker with an empty tracker: the window shows no rows and the
   four buttons, and the table shows no strip. GM adds four pawns from the
   dialog numbered 17, 12, 20 and 12: the strip appears on both screens in
   that order with portraits. Sort: 20, 17, 12, 12, with the two 12s in the
   order they were added.
3. GM presses Next turn twice; the second entry is highlighted on both screens
   and the round reads 1. The player whose pawn it is sees the timer counting
   and End turn; the other player sees neither. At sixty seconds the digits
   turn warning. The player ends the turn: the third entry is active on both
   screens and the timer is gone.
4. Put a two-turn condition clearing at end of turn on the third pawn and go
   round the table twice: the chip under its entry says one turn, then it is
   gone from the strip and from the rings on the table at the same moment.
5. GM hides the third pawn: its entry leaves the player's strip and stays,
   badged, in the GM's. GM reveals it: it comes back in place. GM removes the
   active pawn from the table: the turn moves to the next entry without a
   click.
6. GM clicks an entry in the strip: it becomes active and the round does not
   change. GM starts typing a number in the editor while a second tab
   advances the turn: the field keeps what was typed until it is left, and
   the row catches up afterwards.
7. Two GM tabs: reorder in one; the other's editor and strip follow with no
   reload.
8. Clear tracker: the confirm names it; the strip vanishes on every screen.

## Out of scope

Drag-to-reorder, rolling initiative for monsters, a turn-start sound or
toast, a server-side turn clock, editing a named entry's text, per-entry
notes, and fog, which is phase 7.
