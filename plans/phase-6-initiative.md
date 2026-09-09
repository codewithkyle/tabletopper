# Phase 6: initiative

Read `plans/vtt-overview.md`, then `plans/phase-2-protocol-core.md` for the
initiative commands, `plans/phase-3-transport.md` for the refetch pattern, and
"Windows, and how live data reaches them" at the end of
`plans/phase-5-pawns.md`. This phase runs the fight: everybody sees the turn
order, the GM builds it with one press and drags it into shape, and the player
whose turn it is ends it.

This was the last quarter of a plan that also held fog, strokes and pings. That
plan was split on 2026-09-09, after phase 5 shipped, into this one, phase 7
(fog) and phase 8 (strokes and pings). Nothing in it was built before the
split.

## Two revisions, both before any of it was built

**2026-09-09, first pass.** `hub.Initiative` was answering
`Project(role).Pawns`, which is decision 20 and was a bug; the strip's density
became the role's; the timer and the strip's scroll became one client module;
`N` became the turn key.

**2026-09-09, second pass, and it is a reshape rather than a revision.** The
first pass designed a spreadsheet: a window of rows, a number box per row, Up
and Down buttons, and a Sort. That is not what a turn order is. A turn order is
a row of faces, and this rewrite makes the face the whole design.

What that changed:

- **The card is an avatar.** Every monster, NPC and player in this app has a
  picture, and the picture is the only thing at a table anybody actually reads
  off a turn order. Decisions 2 and 3.
- **The card wears the creature's wounds.** Blood, pallor, a pulse and a skull,
  the same as the sprite on the table and from the same numbers. Decision 3 and
  the whole of "The card" below.
- **The numbers are gone.** No initiative box, no Sort, no renumber route. The
  GM drags. Decision 10.
- **The Tracker window is gone.** The strip is the GM's whole surface: drag to
  reorder, click to activate, right-click to remove. Decision 1.
- **Sync tracker is back**, which was the old client's name and the old
  client's gesture, and it is now how a fight starts and how it grows.
  Decision 9.
- **An entry names a list of pawns**, so that grouped combat can exist at all.
  This breaks the first pass's promise that the phase touches nothing below the
  socket. Decision 7 and the section after this one.
- **Grouped or individual is a table setting**, grouped by default, changeable
  mid-session. Decision 8.
- **The dead are skipped and players are not skipped.** Decision 12.

## Built on 2026-09-09, and the eleven places the build moved

Everything below is in the tree and green under `make check`. What follows is
only where the code and this document stopped agreeing, so that the next reader
trusts the code.

1. **Decision 17's premise was wrong and nothing was done about it.** `tools.ts`
   already calls `preventDefault()` on every space-bar keydown, which is what
   stops a focused `<button>` from reading it as a press -- so Space pans and
   the focused card does not re-activate, which is exactly what verification
   step 11 asks for. `keys.ts` gains no `pressing()` and `tools.ts` is
   untouched.
2. **Clear tracker is `POST .../initiative/clear`**, not a DELETE.
   `RoomMenuItem` speaks `hx-post` and nothing else, and making this the one
   exception would be a second branch in the item markup for one route. It is
   Clear tabletop's reasoning one menu along.
3. **Removing a line presses a hidden per-entry button** carrying
   `hx-delete=".../initiative/{entry}"`, rather than the menu writing a URL onto
   a shared one. htmx reads the verb attribute when it PROCESSES an element, so
   a URL set afterwards is a URL htmx never sees -- the same reason
   `pawn-menu.ts` presses buttons instead of building requests.
4. **The drop posts `entries` as one comma-joined field**, because `hx-vals`
   sets a key rather than appending it. It is `pawnIDs`' shape one file over.
5. **The strip guards the click that follows a drag.** A press and a release on
   the same element fire a click however far the pointer travelled between them,
   and on this strip that click would hand the turn to whatever was just
   dragged. A capture-phase listener on the mount throws it away within 250ms of
   a drop.
6. `data-drag` became **`data-reorder`**, because `data-drag` is a substring of
   `data-dragging` and two attributes on one element that differ by five letters
   are two attributes somebody will mix up.
7. **The wound treatment needs element hooks, so it is more than five rules.**
   The desaturation has to land on the FACE and not on the blood over it, and
   two pseudo-elements cannot hold a splatter, a rim, a pulse and a skull -- so a
   portrait is `[data-portrait]` wrapping `[data-face]`, `[data-pulse]` and
   `[data-skull]`, with `::before` the splatter and `::after` the rim.
8. **The splatter is a blood-coloured radial gradient masked by the splatter's
   own alpha**, rather than two masks composited. The paint is transparent
   through the middle, so the intersection with the rim mask falls out of the
   paint instead of needing `mask-composite` -- one mask and one gradient, both
   supported everywhere.
9. **Sync refuses an empty result with a sentence.** A GM who presses it with
   nobody on the table gets "There is nobody on a floor a player is standing on"
   rather than a button that appears to do nothing.
10. **Sync only ever takes corpses out.** A creature that has been hidden, or
    has walked off the party's floor, keeps its place -- otherwise the next sync
    would undo the Add to initiative that put it there.
11. **Add to initiative is on the pawn window's header as well as its menu**,
    and the controller applies the same group-joining rule Sync does through an
    exported `room.GroupKey` / `room.MonsterKey` pair, so there is one
    definition of what makes two goblins the same goblin.

## Already built

- `internal/room/initiative.go`: `Initiative{Entries, Active, Round}` and
  `InitiativeEntry{ID, PawnID, Name, Initiative}`. Slice order is turn order.
  `initiative.set` replaces the tracker, mints ids, refuses a `pawnId` that is
  not on the table and an `active` that is not an entry, and leaves `Round`
  alone. `initiative.next` wraps, counts rounds, ticks conditions on the two
  pawns at the seam and emits `pawn.updated` for each pawn it changed.
  `initiative.clear` empties it. `InitiativeMax` is 200.
- `initiative.updated` goes ToAll carrying both copies; `projectInitiative`
  drops the entries of hidden pawns from the players', keeps entries for pawns
  on another floor, and clears `active` when the active entry was dropped.
- `pawn.remove`, `table.removeLayer` and `table.clear` drop entries through
  `dropEntriesFor`, which moves `active` on to the next combatant.
  `pawn.setVisible` re-emits the tracker to players when it names the pawn.
- `internal/room/snapshot.go`: `hpBand` and its six bands -- healthy, bruised,
  bloody, veryBloody, nearDeath, dead -- and `projectPawn`, which withholds a
  monster's armour class from players and **sends its hit points**, because the
  canvas draws blood from the number.
- `js/room/render/wounds.ts`: `bandOf`, `healthOf`, `hurt`, `beats`, `bleeds`,
  `BLOOD_FRESH`, `BLOOD_DRIED`, `BEAT_PERIOD`, `SLOW_PERIOD`. This is the
  vocabulary the card borrows and it is already the mirror of the Go.
- `panels.ts` raises `room:initiative` for `initiative.updated` and on every
  snapshot; the reducer applies it.
- Windows, the content modal, `hx-confirm`, `hub.Table`, `hub.Pawn`,
  `room.ProjectedPawn`; in the controllers `gmTable`, `pawnActor`,
  `rejectCommand`, `layerCommand`, `PanelFormErrors`; `pawnPortrait` and
  `characterInitial` in `templ/pages/room-pawn.templ`; `pawn-menu.ts`, whose
  `<template>`-and-`hx-vals` pattern the card menu copies; `keys.ts`'s `typing`
  guard; `RoomGridData` and `gridRadios`, which is where the new setting goes.
- The GM's bar has an Initiative menu with two disabled lines.

## What this phase changes below the socket

The first pass promised none of this and the promise did not survive grouped
combat. Four changes, and they are the reason decision 7 is written at the
length it is.

1. **`InitiativeEntry.PawnID *ulid.ULID` becomes `PawnIDs []ulid.ULID`.** A
   solo entry holds one, a group holds many, a named entry -- a lair action --
   holds none.
2. **`initiative.next` skips.** An entry every one of whose pawns is dead, and
   none of whose pawns is a player's, is passed over. Decision 12.
3. **`Table` gains `InitiativeGrouping`**, and `TableSetOptions` gains the
   field beside `PawnLabels`. Decision 8.
4. **`initiative.sync` is a new command.** It is the one piece of this that
   could have been a controller reading and dispatching `initiative.set`, and
   it is not, for the reason in decision 9.

`projectInitiative`, `dropEntriesFor`, `hasEntryFor`, `tick` and
`InitiativeNext.Authorize` all move from "the entry's pawn" to "the entry's
pawns", which is a loop where each was a nil check.

**Snapshots written before this ship come back with empty trackers**, because
`pawnId` is not `pawnIds` and the old key is dropped on decode. Rooms are
in-memory with a debounced save and this is a rewrite in flight; the cost is a
tracker rebuilt with one press of Sync, and Sync is the press this phase adds.
It is not worth a migration.

## End state

- While the tracker has entries, everybody sees a strip of cards across the top
  of the table. A card is a portrait, a name under it, and nothing else it does
  not need. The active card is bigger and highlighted. An empty tracker is no
  strip at all.
- **A card looks like its creature looks.** Blood soaks in from the rim of the
  portrait as it is hurt, the colour drains out of it, a glow inside the rim
  beats slowly and then fast as it nears death, and a corpse goes grey under a
  skull. It is the same scale, from the same numbers, as the sprite on the
  table.
- The GM presses **Sync tracker** and the fight is in the order: every visible
  creature on every floor a player is standing on, players individually,
  monsters grouped or not by the table's setting. Pressing it again mid-fight
  brings in the reinforcements and takes out the corpses.
- In **grouped** mode, one Goblin War Chief, three Bugbears and nine Goblins is
  three cards. The goblin card carries nine pips, one per goblin, each in the
  colour of that goblin's health, and they grey out as they die.
- The GM **drags cards** into the order they rolled, clicks one to make it
  active, right-clicks one to take it out, and double-clicks a solo one to open
  that pawn's window.
- `N` advances the turn for the GM, or ends it for the player whose turn it is,
  and does nothing for anybody else. **It skips the dead** -- but never a
  player, who is making death saves.
- The player whose pawn is active sees, in that card, a count-up timer in
  `MM:SS` that turns warning at one minute and danger at two, and an End turn
  button. Nobody else sees either.
- Conditions with durations tick down as turns pass, and the chips on the
  active card and the rings on the table agree because both read the pawn.

## Decisions

### The surface

1. **The tracker is a strip over the table, and for the GM it is the whole
   surface.** The rule that panels which are not the table are floating windows
   was made about the player list: a column that took a fifth of the table for
   something read once an hour. The tracker is read by everybody, every few
   seconds, for exactly the minutes a fight lasts, and a window would have to be
   opened by each person at the table from a menu the players do not have. So it
   is a fragment positioned over the table's top edge that renders itself
   `hidden` when the tracker is empty.

   **And there is no editor window beside it.** The first pass had one, because
   the first pass had numbers to type and rows to push up and down. With the
   numbers gone the window's whole remaining content was the same cards in a
   column, and a second copy of the cards is a second thing to build, to style,
   to keep live and to pin in tests -- for the privilege of dragging vertically
   instead of horizontally. The GM edits the thing they are already looking at.
   The Initiative menu carries the four verbs that are not a gesture on a card:
   Sync tracker, Add entry, Next turn, Clear tracker.

2. **The card is an avatar, and everything else on it is subordinate.** This
   rebuild is built on every creature having a picture -- monsters carry the
   manual's, NPCs and players carry an avatar, and `pawnPortrait` already falls
   back to an initial in a disc. A turn order is scanned, not read: the question
   is "whose go is it and who is next", and a face answers it in the time a name
   takes to be focused on. The old client got this right and it is the one part
   of it worth carrying forward whole.

   So: a **round portrait**, because the pawn on the table is a disc and the
   card is the same creature; the name **under** it, truncated, small; and
   nothing else on a resting card. The active card is larger and gains what the
   turn needs. Everything the GM might also want -- hit points, conditions,
   floor -- is one double-click away in the pawn window, which already exists.

3. **The card wears the creature's wounds, and it reads exactly what the canvas
   reads.** "Mimic the pawn" is not a metaphor here: `wounds.ts` already
   exports the whole vocabulary and it is already the mirror of `hpBand` in the
   Go, so the card can be driven from the same band and cannot drift. The full
   spelling is "The card" below.

   **This leaks nothing, and that is settled rather than assumed.**
   `projectPawn` withholds a monster's armour class from players and sends its
   hit points on purpose -- "the blood on it, the blood under it, the pallor and
   the heartbeat all read the number, so the number has to be in the browser for
   the table to look right". A bloodied card is the bloodied sprite in a second
   place. What the label setting still governs is the **text**: the numbers or
   the word under a card, which follows `ExactHP` exactly as the pawn panel
   does.

4. **The strip is a refetching fragment, and the client owns four things in
   it.** The placeholder `room.templ` renders carries `hx-trigger="load,
   room:initiative from:window"`; the fragment's own root carries the event
   alone, because a root that also said `load` would fetch itself again on
   every swap. Both carry `hx-sync="this:queue last"`. The server renders the
   strip for the requester: their role decides whether cards are draggable and
   whether hit points are printed, their user id decides whether the active
   card carries a timer and an End turn.

   The four the client owns are the timer's digits, the strip's scroll offset,
   the drag, and the `N` key -- none of which is room state, and all of which
   are in `initiative.ts`.

5. **Every control is HTTP, End turn and the drag included.** One POST is one
   round trip, which is what a socket frame is too, and HTTP gives every one of
   them the alert modal for a refusal without a line of client code. The drag
   posts through a hidden htmx button whose `hx-vals` the module fills, which is
   the pattern `pawn-menu.ts` and the selection overlay already use and is what
   keeps `hx-confirm` working where it is wanted.

### The model

6. **`initiative.set` is still the only editing command, and now more so.**
   Sync, reorder, add, remove and activate are five routes that each read the
   live tracker, change one thing and dispatch the whole. Phase 2 made set the
   one editing command on purpose. The read and the dispatch are two trips into
   the actor and a second GM tab can slip between them; the loser's edit is
   overwritten by the winner's, which is the race two tabs already have on the
   layer manager and is harmless.

   `initiative.sync` is the exception and decision 9 says why.

7. **An entry names a list of pawns.** `PawnID *ulid.ULID` becomes `PawnIDs
   []ulid.ULID`: one for a solo card, nine for the goblins, none for a lair
   action.

   **Grouped combat cannot be expressed any other way, and the alternatives
   were each tried on paper first.** A *representative pawn* -- the entry keeps
   one id and the group is re-derived around it -- makes the card lie the
   moment the representative dies, and makes the count a query rather than a
   fact. A *render-time collapse* -- thirteen entries drawn as three cards --
   leaves `initiative.next` stepping through nine goblins one at a time behind
   a card that says nothing changed, which is the opposite of what grouped
   means. A *parallel group table* is a second place for the same relationship
   to be wrong in.

   One list, and a solo entry is a list of one. Every loop over it is a loop
   that already had a nil check in it.

8. **Grouped or individual is a table setting, it applies to monsters, and it
   is read at sync time.** `Table.InitiativeGrouping` beside `PawnLabels`,
   default **grouped**, on the Grid and settings window's radios.

   **The default is grouped because it is how most tables run most fights**:
   nine goblins on nine counts is nine turns of bookkeeping for one decision.
   Individual is there because some fights deserve it, and the reason it is a
   setting rather than a build-time choice is that which kind of fight this is
   changes between fights -- so it is changeable mid-session, like every other
   table setting.

   **It applies to monsters and not to NPCs.** The kinds exist to draw that
   line: an NPC is a named individual -- the captain, the informant -- and
   grouping two of them under one card would be the app deciding they are
   interchangeable. Players are never grouped, obviously.

   **The group key is the monster id where there is one, and the name and image
   where there is not.** A pawn spawned from the manual carries `MonsterID` and
   that is the identity; a "Goblin" token dragged out of the asset library has
   none, and two of them are the same creature exactly when a table would say
   they are. The key is recomputed from an entry's members rather than stored,
   so nothing has to be kept in step.

   **Changing the setting does not reshape a tracker that already exists.** A
   fight regrouping itself under the GM's hands mid-round is worse than a fight
   they chose to rebuild: Clear and Sync is two presses and is unambiguous.

9. **Sync tracker is how a fight starts and how it grows, and it is a command
   rather than a controller.**

   ```
   floors     = layers holding at least one visible player pawn
   candidates = visible creature pawns on those floors
   ```

   Objects are not creatures and are not offered. Then:

   - **Corpses go.** A dead monster or NPC is dropped from the tracker and not
     re-added. A **dead player pawn stays**, because a player at zero is making
     death saves and is still in the fight -- which is the same exception
     decision 12 makes for the `N` key, written in both places because it is
     the same fact about the same rule.
   - **Everything already in the tracker stays where it is.** Sync never
     reorders. A GM who has dragged the order into shape and presses it again
     to bring in reinforcements gets their order back with more on the end.
   - **New pawns append**, in the order the table holds them, at the end.
   - **In grouped mode a new monster joins an existing group with its key** if
     there is one, rather than appending. Three more goblins arriving in round
     four are more goblins, not a second goblin turn.

   **It is a command and not a controller** because every clause above is a
   rule about room state that the room is the only thing holding: which floors
   have players on them, which pawns are visible, which are dead, what the
   grouping setting is, and what is already in the tracker. A controller would
   read all of that through `hub.Initiative`, decide, and dispatch
   `initiative.set` -- and the window between the read and the dispatch is
   wider here than anywhere else in the phase, because the read is the whole
   table. `initiative.sync` is a dozen lines in `initiative.go` beside the
   rules it depends on, and it is atomic on the actor's goroutine.

10. **The order is dragged, and the numbers are gone entirely.** No box on a
    card, no box in a dialog, no Sort button, no renumber route, no move-by-index
    route. The slice order was always the turn order and the number was always
    informational; with a drag there is nothing left for it to inform. A GM who
    rolls on paper drags into the order they read off the paper, which is one
    gesture instead of twelve keystrokes and a press.

    `InitiativeEntry.Initiative` stays in the struct at zero. Rolling
    initiative for monsters is a feature this app will want, and it is the
    field it will want; removing it would be a protocol change to make now and
    a protocol change to undo later.

11. **The drag is SortableJS.** It is a dependency and the repo hand-rolls
    things, so this is the exception and it is argued rather than assumed. The
    old client used it for this exact surface. It handles touch, autoscroll
    near the edge of a scrolling strip, and the drop animation -- three things
    that are each a small pile of pointer arithmetic whose bugs surface at a
    table rather than in a test. `esbuild` bundles it from `node_modules` like
    `vanilla-colorful`; it is roughly twelve kilobytes gzipped.

    **Its class names are ours and they live in `app.css`.** `ghostClass` and
    `chosenClass` are options, so nothing is written into the DOM that we did
    not name -- and they go in `app.css` rather than a `.templ` file, because
    `server/js` is not a Tailwind source and a class named in a script is never
    emitted. This is the same rule `window.ts` follows.

    **It is re-mounted after every swap.** The strip is replaced wholesale by
    htmx, so `htmx:afterSettle` destroys the instance and creates a new one on
    the new element. The old client created one per render and never destroyed
    any; that is the bug not to reproduce.

    **And the strip does not refetch mid-drag.** The root's trigger filter is
    `room:initiative[!this.hasAttribute('data-dragging')] from:window`, with
    the attribute set in `onStart` and cleared in `onEnd` -- the same shape as
    the pawn panel's `typingInPanel`, for the same reason: a refetch that
    replaces the list under a held pointer is a dropped card. An update that
    arrives during a drag is lost, and the POST on drop brings a fresh one back
    a moment later.

12. **The dead are skipped, and a player is never the dead.** `initiative.next`
    passes over an entry when it has pawns, every one of them is dead, and none
    of them is a `PawnPlayer`.

    **The exception is death saving throws.** A monster at zero is finished and
    a turn spent on it is a turn wasted. A player at zero has the most
    consequential turn of their character's life -- three saves against three
    failures -- and an app that skipped it would be an app that killed
    somebody's character by omission. The rule is one clause and it is the
    whole difference between the two kinds of creature this app draws.

    A group with one goblin still standing is not skipped, which is what makes
    the count on the card matter. A named entry is never skipped. **And if
    every entry would be skipped, the next one is taken anyway** -- a tracker
    full of corpses advances by one and increments the round rather than
    spinning, because a GM who presses next on a finished fight should get a
    press, not a hang.

    **The round counts the seam and not the skips.** Crossing the end of the
    list increments the round once, however many entries were passed over on
    the way, including a lap that passes over all of them.

    **The GM can still activate a corpse** by clicking its card. Skipping is
    what the button does, not a rule about what may be active.

13. **Removing the active entry moves the turn to the next one in the old
    order**, wrapping, or to nothing when it was the last. It is what
    `dropEntriesFor` does when the pawn itself is removed, written out once
    more in the handler because here the entry is going and the pawn is
    staying.

14. **A card is drawn from its pawns and not from its name.** The entry carries
    a name because a named entry has nothing else, but a goblin renamed
    mid-fight should read the same on its card as on the table. So a card
    prints its pawns' current names, portraits, health and conditions. The
    entry's own name is what a **pawn-less** entry is drawn from -- the lair
    action, the legendary action, the thing that acts on a count and is not a
    creature. It is not a fallback for a missing pawn: `initiative.set`
    validates every id and `dropEntriesFor` removes what is deleted, and
    decision 20 is what makes that true for a player as well.

    A group's name is the group's, taken from the first member at sync and
    reprinted from the members afterwards.

### The turn

15. **The timer starts when this browser sees the active entry change, and the
    active player alone sees it.** That is the honest reading of the one clock
    the client has. A reload starts it from zero: a turn that began before the
    tab was opened is a turn whose length this tab did not watch. Its tones are
    plain, warning from one minute and danger from two -- two steps rather than
    the old client's three, because a "safe" green for the first thirty seconds
    answered a question nobody was asking.

    **The GM does not get a copy.** A clock the whole table can read is a
    stopwatch on whoever is thinking, and the GM already knows a turn is
    dragging without being handed a number to quote.

    It runs on a one-second interval **and writes again on
    `htmx:afterSettle`**, because every swap replaces the span with an empty
    one and an interval-only clock is blank for up to a second after each
    refetch. The first tick that finds no element stops the interval.

16. **The strip hangs from the top edge, grows downward, and keeps the active
    card in view.** The active card is taller than its neighbours, so anchoring
    at the top means nothing already on screen moves when the turn advances; an
    anchored-centre strip would shift every card a few pixels every turn, which
    is the kind of jitter that is never noticed as a bug and is always felt.

    A twelve-combatant fight is wider than the strip, and a turn order whose
    current turn has scrolled off the side is a turn order nobody can read. The
    client scrolls it into view on the change and again after each swap, because
    a swap resets the scrollbar -- **and never while a drag is in progress**,
    which is the same `data-dragging` gate decision 11 puts on the refetch.

17. **`N` advances the turn by clicking the button already on the screen.**
    `V`, `H`, `M`, Space, Escape and Delete are taken; `N` is free and is the
    first letter of the thing.

    **Clicking the rendered button rather than posting** is what makes it one
    binding for two roles with no rule of its own: `initiative.next` is
    authorized for the GM or the active pawn's owner, so the button exists on
    exactly the screens where the key should work. A player pressing `N` out of
    turn finds no button and nothing happens, instead of a 403 in the alert
    modal; nobody gets anything outside a fight, because the strip is `hidden`.
    The client learns no route and no rule.

    **Space needs a guard it does not have.** `keys.ts` skips a key pressed in
    an `INPUT`, `TEXTAREA` or `SELECT`. A `<button>` is in none of those, so
    after the GM clicks a card -- which leaves it focused -- Space both
    re-activates the card and pans the camera. `keys.ts` gains
    `pressing(target)` for the elements the space bar activates, and `tools.ts`
    consults it.

18. **A card's gestures are the table's gestures.** Click activates,
    double-click opens the pawn window for a solo card, right-click puts up a
    small menu, drag reorders. Double-click and right-click are what a pawn on
    the canvas already answers to, so the card answering the same way is one
    thing to learn rather than two.

    The menu is a `<template>` cloned by the client and filled per card, which
    is `pawn-menu.ts`'s pattern and not `pawn-menu.ts` itself -- that one is
    built around one pawn, its floors and its window, and a group card is nine
    pawns and a named card is none. It carries **Remove from tracker** always
    and **Open pawn panel** on a solo card.

    **Removing a card is not confirmed.** It is undone by pressing Sync, which
    is one press away in the menu above it. Clear tracker keeps its confirm,
    because Clear is the whole fight.

19. **Adding by hand is two small things, because Sync is the main road.** Sync
    covers every creature a player can see on a floor they are standing on,
    which is the fight. What it cannot reach is a **named entry** -- a lair
    action on count 20 -- and a creature the GM wants in the order that Sync
    would not take: one that is hidden, or standing on an empty floor waiting
    to burst in.

    The first is `Add entry` on the Initiative menu: the content modal, one
    field, closes on success. The second is **Add to initiative** on the pawn's
    own right-click menu and in the pawn window, which is where the GM already
    is when they are looking at that goblin -- and is a smaller thing to build
    than the checklist dialog the first pass had, which duplicated Sync badly.

### The data

20. **`hub.Initiative` answers the projected tracker and the pawns it names,
    and the gate on those pawns is `Visible` alone.**

    **It must not be `Project(role).Pawns`,** which was the first pass and is
    wrong. `Project` filters a player's pawns through `State.Shown`, which is
    `p.Visible && p.LayerID == s.Table.ActiveLayer` -- and `projectInitiative`
    deliberately KEEPS the entry of a pawn on another floor, because a creature
    that walked downstairs still has a turn. The two filters disagree, and the
    shape of the disagreement is a player watching the party member who went up
    the stairs turn into a nameless card with no portrait for the rest of the
    fight. That is precisely the case decisions 9 and 14 exist to serve.

    So the tracker and its pawns are computed in one pass, in one place, and
    handed back together, which is what makes them unable to drift.

21. **The strip refetches when a pawn it names changes.** Damage arrives as
    `pawn.updated`, which raises `room:pawn` with an id and not
    `room:initiative` -- so without this the blood on the cards would be stale
    until the turn advanced, which is the one thing decision 3 cannot afford.

    `panels.ts` keeps the set of pawn ids the tracker names, refreshed from
    every `initiative.updated` and every snapshot, and raises `room:initiative`
    for a `pawn.updated` or `pawn.removed` that hits it. It is four lines and it
    is the panel-side mirror of the filter each pawn window carries: a window
    cares about one id and asks in its markup, a strip cares about a set and is
    asked for.

    **It stays inside the hot-path rule.** Hit points, conditions and names
    change a few times a round; `pawn.moved` and `pawn.dragging` are not in the
    table and `panels.test.ts` asserts they raise nothing.

## The card

The visual spec, because "mimic the pawn" is only worth saying if the numbers
match. Every value below is the one `wounds.ts` already uses, and the reason it
can be CSS rather than JavaScript is that all of it hangs off one attribute the
server renders.

### The band drives everything

The server puts `data-band` on the card, from the same band the canvas reads:
`p.HPBand` when that is what the viewer was given, otherwise the band computed
from `p.HP` and `p.MaxHP`. That is `healthOf` in `wounds.ts`, and it gets a Go
twin -- `room.Health(p Pawn) *HPBand` -- written beside `hpBand` with a note in
each pointing at the other, exactly as `bandOf` and `hpBand` already do.

A card with no band at all -- a creature nobody told this viewer anything about
-- carries no attribute and is drawn plain.

### What each band does to a portrait

| Band | Rim blood | Colour | Pulse | Splatter | Skull |
| --- | --- | --- | --- | --- | --- |
| healthy | -- | -- | -- | -- | -- |
| bruised | -- | -- | -- | -- | -- |
| bloody | 0.45 | 0.45 | -- | -- | -- |
| veryBloody | 0.75 | 0.75 | slow, 1800ms | fresh | -- |
| nearDeath | 1 | 1 | heart, 1050ms | fresh | -- |
| dead | -- | full grey | -- | dried | yes |

Those are `hurt()`, `beats()` and `bleeds()` unchanged. **Nothing above the
halfway line is marked at all**, which is `hurt`'s most important entry and the
reason a table full of lightly-wounded creatures does not read as a bloodbath.
**A corpse is not rim-marked either**: the grey and the skull say more, and a
red rim under a full grey would be greyed away in the same breath.

### How it is drawn

Five rules in `server/css/app.css`, on `[data-band]`, none of them in a `.templ`
file:

- **A `--wound` custom property**, set per band to the numbers above, and a
  `::after` on the portrait holding a radial gradient from transparent at the
  middle to `BLOOD_FRESH` at the rim, at `opacity: var(--wound)`, biased
  downward so it pools toward the bottom of the disc as the shader's does.
- **`filter: saturate()`** falling with `--wound`, and `grayscale(1)` with a
  brightness cut on `dead`. This is the shader's `mix(rgb, luma, grey)` in the
  one form CSS has of it.
- **Two `@keyframes`**, at 1800ms and 1050ms, animating an `inset box-shadow`
  in `BLOOD_FRESH` on the inside of the rim. Fixed geometry, brightness only:
  a ring that travelled would arrive on the face at the end of every beat,
  which is the one part of a portrait that has to stay legible. Both are
  wrapped in `@media (prefers-reduced-motion: no-preference)`, and the reduced
  case keeps the glow at its bright end rather than dropping it -- the pulse is
  information, and the animation is only how it is delivered.
- **The splatter**, for the three bands `bleeds()` names, as a
  `background-image` from `/images/blood/{1-9}.webp` -- the same nine files the
  sprite cache loads -- under a radial `mask-image` that is transparent to 25
  percent and opaque from there out. That mask is the CSS port of the shader's
  rim mask and it is doing the same job: a splatter is dense in the middle,
  which is exactly where a face is. Tinted `BLOOD_FRESH`, or `BLOOD_DRIED` on
  a corpse, because a creature's blood dries the moment it dies and bright
  arterial red over a grey portrait reads as paint.

  **The variant is chosen from the pawn id, and it is not the one the canvas
  chose.** Matching would mean mirroring `seed()` in Go for a difference that
  is invisible at forty-eight pixels; what the nine are for is variety across a
  strip of twelve cards, and any stable per-pawn choice gives that.

- **The skull** is the 💀 glyph at 70 percent of the portrait's diameter, which
  is `SKULL_SCALE`. It is not an asset: `drawSkull` in `sprites.ts` renders that
  same character to a canvas at 0.8 of a sprite, so a span on the card and the
  glyph on the table are the same typeface doing the same thing, and neither
  needs a file.

Blood colours are `BLOOD_FRESH` `rgb(255 33 26)` and `BLOOD_DRIED`
`rgb(87 15 13)`, named as CSS variables beside the table tokens so the two
files quote one source.

### The three shapes of card

**Solo.** Portrait, name under it. `Hidden` badge for the GM on a hidden pawn.
For the GM, the hit-point text under the name -- numbers or the band's word,
through `ExactHP`; for a player, that text only when the card is active.

**Group.** The monster's portrait, the group's name, and **a row of pips, one
per member**. A pip is the same `data-band` treatment at pip size, so six up
and three down is six coloured discs and three grey ones and there is one rule
set doing both. The portrait itself is drawn from the group's **worst-hurt
living member**, so a group in trouble looks like it; when every member is
dead the whole card takes the corpse treatment.

**Past twelve members the pips become a count** -- `x20` -- because twenty
four-pixel discs under a portrait is a texture rather than a reading.

A group card carries no condition chips and no hit-point text. Nine goblins
have nine of each, and the rings on the table are where that lives.

**Named.** No portrait: the entry's name in the disc's place, at the size the
portrait would have been, so the strip's rhythm survives. No band, no pips, no
conditions.

### The active card

Larger, highlighted, and it grows downward. It gains its pawn's **condition
chips** in the eight colours the pawn panel already prints -- a solo card's own,
and none on a group. For a player it gains the hit-point text a GM already had.
When the active pawn is **this viewer's**, it gains the `MM:SS` timer and the
End turn button.

The round counter sits at the strip's leading edge and is **everybody's**. Next
turn sits at the trailing edge and is **the GM's alone**.

### Names that may not appear in the markup

Tailwind reads a `.templ` file as text and takes a class-name candidate from
every word, attribute name and attribute value in it, splitting on `:` and `.`.
These are DaisyUI components and each would emit its whole family:

`card`, `list`, `table`, `tab`, `status`, `stack`, `swap`, `indicator`,
`steps`, `timeline`, `countdown`, `chat`, `mask`, `dock`, `diff`, `menu`
(where it is not already deliberate).

**`card` is the trap this feature walks straight into**, because a card is what
this whole document calls them. In the markup an entry is an `entry`: `<div
data-entry>`, `#room-initiative`, `data-entry-pips`, `data-turn-timer`,
`data-turn-tone`, `data-turn-active`, `data-turn-next`, `data-dragging`. The
clock is a `timer`. Run the selector diff from CLAUDE.md after every change
under `server/templ`.

## Server

### `internal/room`

```go
type InitiativeEntry struct {
    ID         ulid.ULID   `json:"id"`
    PawnIDs    []ulid.ULID `json:"pawnIds"`
    Name       string      `json:"name"`
    Initiative int         `json:"initiative"`
}

type InitiativeGrouping string
const (
    GroupMonsters   InitiativeGrouping = "grouped"
    GroupIndividual InitiativeGrouping = "individual"
)

// On Table, beside PawnLabels:
InitiativeGrouping InitiativeGrouping `json:"initiativeGrouping"`

// Health is the band an interface draws a creature's injuries from. It mirrors
// healthOf in server/js/room/render/wounds.ts.
func Health(p Pawn) *HPBand

// Dead is a creature at or below zero with a maximum to be measured against.
func Dead(p Pawn) bool

// InitiativeSync builds or extends the tracker. Decision 9.
type InitiativeSync struct{}
```

`initiative.set` validates every id in every `PawnIDs` and refuses a duplicate
across entries -- one pawn is in the order once. `projectInitiative` drops
hidden pawns from an entry's list and drops the entry when that empties it and
it had pawns to begin with. `dropEntriesFor`, `hasEntryFor` and `tick` loop
where they had a nil check; `tick` ticks every member at the seam and reports
every pawn it changed. `InitiativeNext.Authorize` lets a player advance when
they own **any** pawn in the active entry. `InitiativeNext.Apply` skips per
decision 12.

`TableSetOptions` gains the field and validates it, and, like `PawnLabels`, a
change re-emits nothing extra -- grouping is read at sync and changes no
projection.

### `hub.Initiative`

```go
type InitiativeView struct {
    Initiative room.Initiative         // projected for the role
    Pawns      map[ulid.ULID]room.Pawn // the pawns it names, projected
    Table      room.Table              // for pawnLabels and the floor names
}

func (h *Hub) Initiative(ctx context.Context, roomID ulid.ULID, role room.Role) (*InitiativeView, bool)
```

A fourth read accessor beside `Players`, `Table` and `Pawn`, answered on the
actor's goroutine through an `initiativeView{role, reply}` message. It LOADS
the room, as `Table` does: the strip is fetched on every page load and the
socket connects a moment later anyway. `drain` answers it with nil like the
others. `internal/room` gains the exported half:

```go
// ProjectedInitiative is the tracker this role may see and the pawns it names.
func (s *State) ProjectedInitiative(role Role) (Initiative, map[ulid.ULID]Pawn)
```

For the GM, `cloneInitiative` and a clone of every pawn an entry names. For a
player, `projectInitiative` and `projectPawn` for each pawn that survived it.
**The gate is `p.Visible`, not `s.Shown(p)`** -- decision 20 -- and it is the
same gate `projectInitiative` applies one loop earlier, which is why they are
written next to each other. The map is keyed by id because the caller looks up
one pawn per member and a slice would be a scan per pip.

### Routes

| Pattern | Wrapper | Handler | Behaviour |
| --- | --- | --- | --- |
| `GET /fragment/room/initiative?room={id}` | `Fragment` | `RoomInitiativeFragment` | Any member. The strip for the requester's role, `hidden` when empty. |
| `GET /fragment/room/initiative/entry?room={id}` | `Fragment` | `RoomInitiativeEntryFragment` | GM only, else an empty 404. The Add entry modal, at `sm`. |
| `POST /rooms/{id}/initiative/sync` | `RequireSession` | `SyncInitiative` | GM only. Dispatches `initiative.sync`. 204. |
| `POST /rooms/{id}/initiative/order` | `RequireSession` | `OrderInitiative` | GM only. Reads `entry`, repeated, the whole order. Reorders the live entries to match and dispatches set. A set of ids that is not exactly the tracker's is a 409 and an alert -- the drag raced a change and the refetch that follows is the answer. 204. |
| `POST /rooms/{id}/initiative` | `RequireSession` | `AddInitiative` | GM only. Either `name` for a named entry or `pawn` for one pawn; appends. In grouped mode a monster joins its group. 422 in the modal's error slot. `htmx.CloseModal` and 204. |
| `POST /rooms/{id}/initiative/next` | `RequireSession` | `NextInitiative` | GM or the active pawn's owner; the core decides. 204. |
| `DELETE /rooms/{id}/initiative` | `RequireSession` | `ClearInitiative` | GM only, behind `hx-confirm`. 204. |
| `POST /rooms/{id}/initiative/{entry}/activate` | `RequireSession` | `ActivateInitiative` | GM only. Set with the same entries and a new `active`. 204. |
| `DELETE /rooms/{id}/initiative/{entry}` | `RequireSession` | `RemoveInitiative` | GM only. Decision 13. 204. |

The per-entry and per-tracker mutations share `initiativeCommand`, which is
`layerCommand` with a tracker read in front of the build: `pawnActor` for the
actor, `hub.Initiative` as the GM for the live tracker, the entry parsed from
the path where the pattern carries one, `htmx.NotFound` for an entry that has
gone, and `rejectCommand` for a refusal. Every mutation answers 204 and redraws
nothing: each ends in `initiative.updated`, and the strip refetches from that.

### Templates

- `pages/room-initiative.templ`, `.go`: `RoomInitiative(data)` and its card
  partials, plus `RoomInitiativeEntryForm(data)` for the Add entry modal, plus
  the `<template>` for the card menu.
- `RoomInitiativeData`: `RoomID`, `IsGM`, `Round`, `Empty`, the paths, and
  `Entries []RoomInitiativeEntry` with `ID`, `Name`, `Kind` (solo, group,
  named), `Image`, `Band`, `HP`, `Active`, `Hidden`, `Mine`, `Solo` (the pawn
  id, for the double click), `Pips []RoomInitiativePip` and `Conditions
  []RoomPawnCondition`. A pip is `{Band string}` and nothing else. The
  controller composes every string and picks every band; the templates print
  them.
- `pages/room.go`: the Initiative menu becomes `Sync tracker` (`Post`), `Add
  entry` (`Modal`, `sm`), `Next turn` (`Post`) and `Clear tracker` (`Post`,
  `Danger`, with a confirm). `RoomPageData` gains the paths. `room.templ`
  mounts the strip's element inside `#tabletop` with its `load` trigger.
- `pages/room-grid.templ`: one more `gridRadios`, `initiativeGrouping`, "How
  monsters take their turns", Grouped and Individual.
- `pages/room-pawn.templ` and the pawn menu: `Add to initiative`, GM only,
  posting to the add route with the pawn id. Decision 19.

## Client

### `initiative.ts`

```
initiative.ts   the strip's four client-side jobs: the timer, the scroll,
                the drag, and the N key
```

`mountTurns(state)` returns `{ changed(): void; stop(): void }`. `main.ts`
calls `changed()` after reducing a `snapshot` or an `initiative.updated`.

**The clock.** `changed()` compares `state.initiative.active` with the value it
last saw: a different one records `since = performance.now()`, the same one
leaves it alone, so a card dragged mid-turn does not restart it. `write()` fills
`[data-turn-timer]` with `MM:SS` and sets `data-turn-tone` to `warning` at sixty
seconds and `danger` at a hundred and twenty. It runs on a one-second interval
and on `htmx:afterSettle`; the first tick that finds no element stops the
interval.

**The scroll.** `reveal()` calls `scrollIntoView({ inline: "nearest", block:
"nearest", behavior: "smooth" })` on `[data-turn-active]`, from `changed()` and
from `afterSettle`, and never while `data-dragging` is set.

**The drag.** On `afterSettle`, if the strip is there and the viewer is the GM,
destroy any previous `Sortable` and create one on it: `draggable:
"[data-entry]"`, `animation: 150`, our own `ghostClass` and `chosenClass`,
`onStart` setting `data-dragging` on the root, `onEnd` clearing it and -- when
the index changed -- writing the new id order into the hidden order button's
`hx-vals` and clicking it.

**The key.** A `keydown` on the document, guarded by `typing`, that on `n`
clicks `[data-turn-next]` if it is there.

No class name is written in this file. `app.css` holds the band rules, the two
keyframes and the two Sortable classes; `room-initiative.templ` holds
everything else.

### `initiative-menu.ts`

The card's right-click menu: clone the `<template>`, fill the two items, place
it inside the table's bounds, close on the next pointer down, wheel or Escape.
It is `pawn-menu.ts`'s shape and about half its length, and it sets `hx-vals`
on hidden buttons and clicks them rather than building requests, so
`hx-confirm` keeps working where it is used. Double-click on a solo card calls
the same `pawnWindow` the canvas's double click calls.

### `panels.ts`

Keeps `tracked`, the set of pawn ids the tracker names, refreshed on
`initiative.updated` and `snapshot`; raises `room:initiative` for a
`pawn.updated` or `pawn.removed` whose id is in it, alongside the `room:pawn`
it already raises. Decision 21.

### `keys.ts` and `tools.ts`

`keys.ts` gains `pressing(target)`, true for a `BUTTON`, an `A` and an `INPUT`
of type `button`, `submit`, `reset`, `checkbox` or `radio`. `tools.ts` skips
its pan for a space bar that `pressing` claims, so a focused card is not also a
camera. Decision 17.

### `package.json`

`sortablejs` pinned to an exact version, beside `vanilla-colorful`. No types
package is needed if the one import is typed locally; if `@types/sortablejs` is
added it is a devDependency.

## Tests

**Go**:

- `hub.Initiative` projects. **The security test, written first**: a player
  asking for a tracker that names a hidden pawn gets no entry for it, a group
  with three hidden members answers six pips rather than nine, and `active` is
  nil when the active entry was dropped.
- `hub.Initiative` answers a pawn **on a floor that is not the active one**,
  for a player, when the tracker names it. Decision 20's regression, and the
  test the first pass would have failed.
- `initiative.sync`: takes visible creatures on floors holding a visible player
  pawn and nothing from an empty floor; drops a dead monster and **keeps a dead
  player pawn**; leaves the existing order alone and appends the new; in
  grouped mode makes one entry per monster key and merges a later arrival into
  it; in individual mode makes one per pawn; never groups an NPC; refuses a
  player.
- `initiative.next` skipping: a dead monster is passed over; a dead player pawn
  is **not**; a group with one survivor is not; a tracker of nothing but
  corpses advances by one and increments the round; a lap that skips every
  entry increments the round exactly once.
- `initiative.set` refuses the same pawn in two entries and refuses an id that
  is not on the table.
- `OrderInitiative` with an id set that is not exactly the tracker's is a 409;
  with the right set it dispatches one `initiative.set` in the given order.
- `RemoveInitiative` of the active entry activates the next in the old order,
  wraps from the last to the first, and clears `active` when the tracker
  empties.
- `AddInitiative` takes a name or a pawn and not both, refuses an object, and
  closes the modal.
- `NextInitiative` from a player who does not own the active pawn is the core's
  `forbidden`, rendered as a 403 alert.
- `room.Health` agrees with `hpBand` for every band and answers the computed
  band for a pawn carrying numbers and no band -- which is the
  labels-off case and the one where the card and the canvas could disagree.
- Template tests pin: the strip's `hx-trigger` and `hx-sync`, that the
  fragment's root does not carry `load`, and that its filter carries
  `data-dragging`; `hidden` when empty; `data-band` on a card matching the
  pawn's band, and on each pip; the corpse card carrying the skull; End turn,
  the timer and `data-turn-next` only on the active card and only for its
  owner; `data-entry` draggable only for the GM; hit-point text on every card
  for the GM and on the active card alone for a player; the round for both
  roles and Next turn only for the GM; the Hidden badge only for the GM; a
  group card with no conditions and with pips capped at twelve; **and that the
  word `card` appears nowhere in the rendered markup**.

**TypeScript**:

- `initiative.test.ts`: `changed()` resets `since` only when `active` differs;
  the tone is plain at fifty-nine seconds, warning at sixty, danger at a
  hundred and twenty; no interval starts without an active entry;
  `afterSettle` writes the digits without waiting for a tick and reveals the
  active card; neither the reveal nor the refetch happens while `data-dragging`
  is set; `n` clicks `[data-turn-next]` and does nothing when there is none or
  when the caret is in a field.
- `panels.test.ts`: `pawn.updated` for a pawn the tracker names raises
  `room:initiative` as well as `room:pawn`; one it does not name raises only
  `room:pawn`; `initiative.updated` refreshes the set; none of `pawn.moved`,
  `pawn.dragging` or `stroke.extended` raises anything.
- `keys.test.ts`: `pressing` is true for a button and false for a `<div>`, and
  a space bar with a button focused does not pan.

## Verification

1. `make js && make check` and the CSS selector diff, which should show only
   selectors the strip and the modal use -- and no DaisyUI `card` family.
2. Empty tracker: no strip on any screen. GM spawns a war chief, three bugbears
   and nine goblins on the party's floor and presses **Sync tracker**. Three
   monster cards plus one per player appear on every screen, players first in
   table order, portraits on all of them. Switch the setting to Individual,
   Clear, Sync again: thirteen monster cards.
3. **The wounds.** Take a goblin to half: its card and its sprite go bloody
   together. To a quarter: both go very bloody and both start the slow pulse,
   in step. To one hit point: both go to the fast pulse. Kill it: both go grey,
   the card takes the skull, and the pip for it in the group card greys out.
   Set pawn labels to off and repeat: the card still bloodies, because the
   canvas still does.
4. Drag the bugbear card to the front on the GM's screen: it moves on the
   player's screen with no reload. Drag a card while a second GM tab advances
   the turn: the strip does not swap under the pointer, and it catches up on
   drop.
5. GM presses **N** twice. The round reads 1 on both screens. The player whose
   pawn it is sees the timer and End turn; the other player sees neither, and
   neither does the GM. At sixty seconds the digits turn warning. The player
   presses **N**: the turn moves and the timer is gone. The other player presses
   **N**: nothing happens and no modal opens.
6. **The skip.** Kill two of the three bugbears and the whole goblin group.
   Press N round the table: the goblin card is passed over, the bugbear card is
   not. Take a player to zero: their card is **not** passed over. Click the
   goblin card: it activates anyway.
7. Put a two-turn condition clearing at end of turn on the war chief and go
   round twice: the chip on its active card says one turn, then it is gone from
   the card and from the rings at the same moment.
8. GM hides one goblin: the player's goblin card drops to eight pips and the
   GM's stays at nine, badged. GM reveals it: back to nine.
9. **The floor case, decision 20.** GM moves a player's pawn upstairs while it
   is in the tracker. On the player's screen the pawn leaves the canvas and its
   card keeps its portrait and its name. Its turn comes round and the timer and
   End turn still appear for its owner.
10. Add four more goblins and press **Sync tracker** again: in grouped mode the
    goblin card goes to thirteen pips and the order is unchanged; in individual
    mode four new cards land at the end.
11. Right-click the war chief's card: Remove takes it out with no confirm.
    Double-click a bugbear card: its pawn window opens. Press Space with a card
    focused: the camera pans and the turn does not move.
12. Add a named entry "Lair action": it lands at the end with no portrait, no
    band and no pips, and N does not skip it.
13. Twenty goblins in one group: the card shows a count rather than pips.
14. **Clear tracker**: the confirm names it; the strip vanishes on every
    screen.

## Out of scope

Rolling initiative for monsters, a group whose members act on different counts,
editing a named entry's text, per-entry notes, drag between the strip and
anything else, a turn-start sound or toast, a server-side turn clock, a turn
timer for the GM, and fog, which is phase 7.
