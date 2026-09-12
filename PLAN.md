# The character sheet at the table

Written 2026-09-12 from a read of `server/internal/room`, `server/internal/hub`,
`server/internal/controllers/{characters,character-panels,room-pawns,room-page}.go`,
`server/templ/pages/{edit-character,character-bar,sheet-section,room}.*` and
`server/js/room/{panels,window,render/decals}.ts`.

Two things are being built, and only one of them is new UI.

1. **A player can open their character sheet in a room window and edit it there.**
2. **Size, AC, maximum hit points and current hit points are one value, not two.**
   Change one in the pawn's details panel and the sheet follows. Change one in
   the sheet and the pawn follows, with the blood, the bands and the heartbeat
   that any other hit point change would raise.

The second is the hard half and it comes first.

## What is already true

Most of the mechanism exists. It is worth being precise about how much, because
it determines how small this can be.

**The pawn already writes hit points back to the sheet.** `hub/effects.go`
watches every `PawnsUpserted` change, and for a `player` pawn carrying a
`characterId` it hands the total to `hub/sheet.go`'s `sheetWriter`, a
coalescing goroutine that calls `UpdateCharacterCurrentHP`. `actor.lastHP`
suppresses a write when the total has not moved. This path is correct; it is
simply too narrow — it carries one of the four values.

**The sheet already fills the pawn, once.** `PawnSpawn.resolveCharacter` and
`PawnSpawnCharacters.Resolve` read the row through `hub/library.go`'s
`Character`, and `room.characterPawn` turns a `CharacterInfo` into a `Pawn`.
That mapping — name, portrait, size, hp, maxHp, ac, owner — is the single
definition of what a character's pawn is made of. Nothing re-runs it after the
spawn.

**Blood and bands need no server work at all.** `render/stages/decals.ts` calls
`decals.watch(frame.state.pawns, …)` on every frame; `decals.ts` remembers each
pawn's `hp` and spawns splatter when it drops. Bands come from
`model/health.ts`'s `healthOf`, computed on the client from `hp`/`maxHp`, and
drive the tint, the heartbeat and the bleed in the pawn pass. **Any change that
reaches the pawn through the ordinary `pawns.upserted` path gets every effect for
free.** There is nothing to wire, and any design that bypasses `pawns.upserted`
throws that away — which is the single strongest constraint on this plan.

**A window is a fragment URL and nothing else.** `server/js/room/window.ts` clones
one template, `htmx.ajax`s a `/fragment/` URL into it, and remembers geometry in
`localStorage`. Anything already served under `/fragment/` is a window with no
client change.

**Sizes already agree.** `pages.sizeOptions` is exactly `room.Size.Values()`:
tiny, small, medium, large, huge, gargantuan. `room.CreatureSize` lowercases and
falls back to medium. No translation table is needed.

**No snapshot schema change and no database migration.** Nothing here adds a
field to `Pawn` or to `State`, so `room.Schema` stays at 3, the fixtures under
`testdata/snapshots` stay as they are, and `protocol.ts` does not move — the new
command is a hub command, and `gen` only renders the wire registry. `characters`
already has `size`, `ac`, `max_hp` and `current_hp`.

## The shape

Four rules. Every phase moves toward them.

1. **The pawn is the live value; the row is the durable one.** While a room holds
   a pawn for a character, that pawn is what the sheet displays for the four
   synced fields, and every edit on either side ends up in both. Once the pawn is
   gone the row is all there is. This removes any need to reason about how far
   behind the asynchronous write-back is.
2. **Both directions go through `pawns.upserted`.** The sheet never reaches into
   the renderer, never posts a "please splat" and never special-cases an edit as
   distinct from damage. It changes the pawn; the table reacts the way it always
   does.
3. **One definition of what a character's pawn is made of.** `room.characterPawn`
   is it, and the new sync command builds through it rather than beside it.
4. **One sheet, rendered at two widths.** The window does not get a second copy
   of the sheet's markup. The page's layout moves from viewport media queries to
   container queries so the same components lay out correctly at 480px inside a
   window and at 1600px on `/characters/{id}/edit`.

## The one hazard worth naming up front

The vitals panel posts all nine of its fields at once. If the GM lands a hit
while a player has the sheet open, the player's next vitals save writes the
**stale** current hit points back over the damage — the form was rendered before
the hit. This is not hypothetical; it is the normal case in a fight.

Phase 4 is the answer, and it is not optional polish: the three panels that carry
a synced field refetch themselves when the pawn changes, guarded so a refetch
never yanks a field the player is typing in. That is the same
`hx-trigger="room:pawn[…] from:window"` pattern `RoomPawnFragment` already uses.

## How each phase is run

- Tests are written or changed first. `make check` is red at the end of that step
  for the reasons the phase names and for no other reason.
- Then the code, until `make check` is green.
- `make check` is `fmt-check vet test js-test`; the room bundle is `make js`.
- No comments, as ever.

---

## Phase 0: one sheet, two widths

The sheet's responsive layout is viewport-driven: `max-[1280px]:grid-cols-1` on
the two-column split, `max-[900px]:grid-cols-1` on nine inner grids, and seven
more in `character-bar.templ`. Inside a 520px window on a 2560px monitor none of
them fire, so the sheet renders as a two-column page squeezed into a panel. That
is the whole reason the window cannot simply render `characterPanels`.

### Tests first

- `server/templ/pages/pages_test.go`: a render test that `characterPanels` emits
  no `max-[` viewport variant and that its root carries a **named** container
  (`@container/sheet`). Red until the conversion happens, and it is what keeps a
  later edit from reintroducing a media query that the window cannot honour.

### Then

- Put `@container/sheet` on the wrapper in `characterPanels` and convert every
  `max-[1280px]:` and `max-[900px]:` inside the sheet to
  `@max-[1280px]/sheet:` / `@max-[900px]/sheet:`. **The container must be named.**
  `edit-character.templ` already nests a bare `@container` around the abilities
  grid, and an unnamed max-variant inside it would resolve against that inner
  container instead of the sheet.
- Same conversion in `character-bar.templ`, against its own named container, so
  the chips row can be reused in the window.
- Extract the sheet body — the tab strip's target — into a component both the
  page and the window fragment render. The page keeps `editCharacterShell`; the
  new component is everything inside it.
- The container goes on `character-editor` in `editCharacterShell`, not on
  `characterPanels`. That element is `w-screen`, so every query resolves against
  the viewport exactly as the media queries did and the page does not shift by a
  pixel; it also puts the bar's chips inside the same container as the panels,
  which the window needs. The window fragment carries its own `@container/sheet`
  on its root.

### Done when

`/characters/{id}/edit` looks as it does today at desktop and phone widths, and
the same component rendered into a 480px box collapses to one column.

---

## Phase 1: the pawn writes the whole seat back

Widen the existing write-through from one value to four.

### Tests first

- `server/internal/hub/sheet_test.go`: the writer carries size, AC, maximum and
  current hit points; a second put for the same character still coalesces to the
  latest; a put whose four values match the last one written is skipped.
- `server/internal/hub/pawn_test.go`: a GM `PawnUpdate` that changes a character
  pawn's size and AC reaches the sheet writer. A pawn that is not a `player`
  kind, or has no `characterId`, or is missing any of the four, reaches nothing.

### Then

- `server/sql/characters.sql`: replace `UpdateCharacterCurrentHP` with

  ```sql
  -- name: UpdateCharacterFromPawn :execresult
  UPDATE characters
  SET current_hp = ?, max_hp = ?, ac = ?, size = ?
  WHERE id = ?;
  ```

  There is no owner clause, as there is none today: the hub writes as the
  server, for a pawn whose ownership the room already established. `make sqlc`.
- `room.SheetVitals{HP, MaxHP, AC int; Size Size}` in the room package — the room
  owns the shape because it is the room's four values.
- `hub/sheet.go`: `pending map[ulid.ULID]room.SheetVitals`, and
  `Options.WriteHP` becomes `Options.WriteSheet`.
- `hub/effects.go`: `writeThroughHP` becomes `writeThroughVitals`, returning the
  four only when `Kind == PawnPlayer`, `CharacterID != nil`, `HP`, `MaxHP` and
  `AC` are all non-nil and `Size.Valid()`. `actor.lastHP` becomes `lastSheet`
  keyed the same way.
- `hub/persist.go`: `sheetHP` becomes `sheetVitals`, clamping to the column
  widths as it already does for hit points. The room's own limits (`HPLimit`
  9,999, `ACLimit` 99) are inside `SMALLINT UNSIGNED` already, so the clamp is a
  belt to the braces.

### Done when

A GM changing a character pawn's size, AC or maximum hit points in the pawn panel
is visible on `/characters/{id}/edit` after a reload.

---

## Phase 2: the sheet writes back to the pawn

The missing direction. A hub command, because it originates on the server after
the controller has already proved who owns the character — it must never be
reachable from the socket.

### Tests first

- `server/internal/room/pawn_test.go`: `CharacterSync` moves the pawn's name,
  portrait, size, AC, maximum and current hit points and leaves its position,
  layer, z, rotation, conditions and visibility alone; it clamps current to the
  new maximum; it updates the seat's `CharacterName`; it is a no-op when no pawn
  exists for that character; it refuses values the pawn's own validation refuses.
- `server/internal/room/wire_test.go`: `character.sync` is in the hub registry and
  `DecodeCommand` refuses it. (`TestTheTwoRegistriesDoNotOverlap` already covers
  the other half.)
- `server/internal/hub/library_test.go`: `Hub.SyncCharacter` on a room that is not
  loaded does nothing and reads nothing.
- `server/internal/controllers/character-panels_test.go`: saving vitals while the
  session holds a room dispatches one sync; saving with no room does not.

### Then

- `server/internal/room/character.go`:

  ```go
  type CharacterSync struct{ Info CharacterInfo }
  ```

  registered as `"character.sync"` in `hubCommands`. `Authorize` returns nil.
  `Apply` finds the seat, updates its `CharacterName`, finds `s.pawnFor(id)`,
  and — this is the point of rule 3 — builds the target with
  `characterPawn(c.Info, seat)` and copies its name, image, size, hp, maxHp and
  ac onto the existing pawn, then `checkPawn`, `clampHP`, `Normalize`. Building
  through `characterPawn` is what keeps the "no portrait, so borrow the seat
  avatar" rule from being written twice.
- `hub/hub.go`:

  ```go
  func (h *Hub) SyncCharacter(ctx context.Context, roomID, owner, character ulid.ULID)
  ```

  returns immediately when `!h.live(roomID)`, otherwise reads through
  `h.library(owner).Character` and `Notify`s the command. The re-read is
  deliberate: the controller has the row in hand, but mapping it a second time in
  the controller would put a second copy of "what a pawn takes from a character"
  next to the one in `library.go`. A primary-key read is the cheaper mistake.
- `controllers/character-panels.go`: `finishCharacterPanel` calls it once, after
  the row it already re-reads for derived values, when `a.Hub != nil` and
  `sess.RoomID != nil`. One call site covers identity (name, size), core stats
  (AC) and vitals (both hit point totals) — and harmlessly no-ops for skills,
  personality and appearance, because `actor.broadcast` compares before and after
  and sends nothing when they match.
- `controllers/characters.go`: the same call after `UploadCharacterAvatar`, so a
  new portrait reaches the token.

### The sheet's own limits move to meet the room's

`buildVitalsInput` and `buildCoreStatsInput` accept anything a `uint16` holds.
The room caps hit points at `HPLimit` (9,999) and armour class at `ACLimit` (99).
Left alone, the two directions fight: a player types 20,000, the pawn clamps to
9,999, and Phase 1's write-back quietly rewrites the row to the clamp. So the
form is held to the room's numbers, and `CharacterSync` clamps as well, because a
row written before this change is not the form's to reject.

### Accepted, not engineered around

A sheet save writes the row, pushes the pawn, and the pawn's change pushes the
same four values back at the row through Phase 1's writer — one redundant
`UPDATE` with identical values per save. It cannot loop: `broadcast` compares
states and emits nothing when they are equal, so the second lap is where it
stops.

### Done when

With the room open in one tab and `/characters/{id}/edit` in another, editing
current hit points on the sheet moves the pawn's bar, drops its band and throws
blood on the table.

---

## Phase 3: the sheet in a window

### Tests first

- `server/routes_test.go`: `GET /fragment/character/sheet` matches its own
  pattern, and `POST` to it falls through to `middleware.FragmentNotFound`.
- `server/internal/controllers/characters_test.go`: the fragment 404s with an
  empty body — never `http.NotFound` — for a session with no character, a session
  whose room is not the one asked for, a bad `room`, and an unknown `section`.
- `server/templ/pages/rooms_test.go`: a player's Character menu carries a window
  item and a GM's menu carries no Character menu at all.

### Then

- `RoomPageData` gains `CharacterID string` and `CharacterName string`, filled in
  `RoomPage` from `roomCharacter(sess, row.ID)` and the `characterName` helper the
  socket handler already uses, plus `SheetPath()` returning
  `/fragment/character/sheet?room=` + ID.
  `characterMenu()` becomes a method and its first `comingSoon` entry becomes

  ```go
  RoomMenuItem{Label: "Character sheet", Window: RoomWindow{
      ID:     "character-sheet",
      Title:  d.CharacterName,
      URL:    d.SheetPath(),
      Width:  520,
      Height: 640,
  }}
  ```

  Journal stays `comingSoon`; it is its own menu item and its own piece of work.
- `GET /fragment/character/sheet` behind `auth.Fragment`, taking `room` (a ULID
  the session must be a member of) and an optional `section` validated against
  `{main, inventory, spells}` with `level` validated against `0`–`9` — the same
  shape as `RoomSpawnFragment`'s `?kind=`. **The character comes from the
  session, never from the query**, so the URL that `window.ts` parks in
  `localStorage` cannot name somebody else's sheet and stays correct if the
  player rejoins as a different character.
- The fragment renders the Phase 0 body component plus `characterBarFigure` and
  `characterBarChips`. Both are included because `finishCharacterPanel` swaps
  both out of band; leaving either out means every save logs an
  `htmx:oobErrorNoTarget`. The figure keeps its `<h1>`: one handler serves the
  page and the window and cannot tell them apart, so a parameterised heading tag
  would be an `<h1>` again after the first out-of-band swap. A second `<h1>`
  naming the character inside a window the dialog already labels is the smaller
  cost.
- The four synced fields render from the pawn when `Hub.CharacterPawn(ctx,
  roomID, characterID, role)` — a new `view` over `state.pawnFor` — returns one,
  and from the row otherwise. This is rule 1, and it is what makes the write-back
  latency stop mattering.

### Done when

A player opens Character → Character sheet, gets their sheet in a draggable
window, edits a field, and sees the toast. Only one can be open: the window id is
fixed, so the sheet's element ids stay unique in the room document.

---

## Phase 4: the window stays live

Without this the sheet is a stale form that quietly reverts damage. See the
hazard above.

### Tests first

- `server/js/room/panels.test.ts`: a `pawns.upserted` change carrying a pawn with
  a `characterId` raises `room:character` with that id; one without raises none;
  a `pawns.removed` raises it for the character the client last saw on that pawn.
- `server/internal/uievents/uievents_test.go` already pins Go against
  `public/js/events.js` and greps for spelled-out names — it will fail until both
  copies of `ROOM_CHARACTER` exist. That failure is the test for this step.
- `server/internal/controllers/character-panels_test.go`: the panel fragment
  returns the vitals panel alone, with current hit points taken from the pawn
  rather than the row when the two disagree.

### Then

- `ROOM_CHARACTER = "room:character"` in `public/js/events.js` and
  `internal/uievents`.
- `panels.ts` dispatches it from `announce`, keeping a `pawnId → characterId` map
  the way it already keeps `tracked` for initiative, so a removal can name the
  character whose pawn went away. It also raises `WINDOW_RETITLE` for
  `character-sheet` when the viewer's own character is renamed; the viewer's
  character id comes off each snapshot's `players` entry for `you.id`.
- `section=panel&panel={identity|core-stats|vitals}` on the sheet fragment,
  returning that one panel.
- A `livePanel` wrapper in `sheet-section.templ`. `savingPanel` is a `<form>` and
  already carries `hx-post`, so the refetch cannot live on it — one element, one
  verb. The wrapper is a `<div>` with `hx-get`, `hx-swap="outerHTML"`,
  `hx-sync="this:queue last"` and

  ```
  room:character[detail.id === '<ULID>' && !<typing guard>] from:window
  ```

  reusing `room-pawn.go`'s `typingInPanel` expression verbatim — same package, and
  the rule it encodes ("never swap a panel out from under a caret") is the same
  rule. When the sheet is rendered outside a room the wrapper renders its children
  bare, so the page is untouched.

### Done when

The GM applies 8 damage; the open sheet's Vitals panel shows the new total within
a frame, the chips row follows, and the player's half-typed maximum hit points are
still there.

---

## Phase 5: inventory and spells in the window

### Tests first

- `controllers` tests for `section=inventory` and `section=spells&level=…`,
  including a rejected level and a rejected section.
- A render test that the inventory and spell components carry no `max-[` viewport
  variant, the same guard Phase 0 put on the sheet.

### Then

- The Phase 0 conversion, repeated for `edit-inventory.templ` and
  `edit-spells.templ`.
- A tab strip in the window body, `hx-get`ting the sheet fragment with a different
  `section` and swapping `#character-sheet-body` with `outerHTML`. The fragment
  root is that element, so one swap replaces the section and repaints the active
  tab. It targets an element inside the window content, so `window.ts`'s
  `htmx:before:swap` guard — which only fires for swaps aimed at the window root —
  does not see it.
- A failed tab fetch has no window error pane to fall into, so it answers through
  `htmx.Error` and the alert modal.

### Done when

A player moves between Character, Inventory and Spells inside the window and
edits in each, without the window losing its place or its size.

---

## Phase 6: a room that was asleep catches up

`Hub.SyncCharacter` does nothing when the room is not loaded, which is right —
there is nobody to tell. But the pawn in the saved snapshot is then behind the
row, and stays behind when the room wakes.

### Tests first

- `server/internal/hub/membership_test.go`: a player joining a room whose pawn
  disagrees with the row ends with a pawn that agrees, and a GM joining reads no
  character at all.

### Then

- `actor.join` kicks off the refresh for `c.player.CharacterID` on a goroutine —
  it must not be the actor's own, since the read is a database round trip and the
  `Notify` it ends in posts back to the same inbox.
- One row read per socket connection, which is the same order as the
  `GetCharacterName` the socket handler already does per connection.

### Done when

Edit the sheet with the room closed, open the room, and the pawn is right without
a respawn.

---

## Phase 7: the effects, end to end

Mostly proof, because rule 2 means there is nothing new to build.

### Tests

- `server/js/room/render/decals.test.ts`: a `watch` sequence in which hit points
  fall for a reason other than a GM's damage entry still spawns splatter, and
  crossing to zero still lays the pool. The renderer only sees `pawn.hp`; this
  test is what says so.
- `server/js/room/model/health.test.ts`: bands at the boundaries, already fixtured
  in `testdata/rules/bands.json` on the Go side and unchanged by any of this.
- A Go test that a `CharacterSync` lowering hit points derives a `pawns.upserted`
  for **both** roles, so the player watching their own token sees it too.

### Manual acceptance

1. Player opens the sheet, sets current hit points to a quarter of maximum: the
   token turns very bloody, the slow beat starts, blood lands.
2. GM sets the same pawn to 0 in the pawn panel: the pool spreads, the sheet's
   Vitals and chips show 0 without a reload.
3. Player raises maximum hit points on the sheet: the band eases, current is
   clamped if it was above the old maximum.
4. Player changes size to Large on the sheet: the token grows and re-snaps.
5. All four survive a reload of the room, and a reload of
   `/characters/{id}/edit`.

---

## Not in this

Named so that nobody has to guess whether they were forgotten.

- **Temporary hit points, death saves, exhaustion and inspiration do not reach the
  pawn.** The pawn has no field for any of them. Death saves at zero hit points
  are the obvious next tie-in and are deliberately left for after this.
- **The GM cannot open a player's sheet.** The fragment reads the character from
  the session, which is what makes it unspoofable; a GM view would need a
  different route and a different authorisation rule.
- **The journal stays a `comingSoon` menu item.** It is a separate window and a
  separate piece of work.
- **Conditions stay pawn-only.** The sheet has no concept of them.
- **Nothing syncs between two rooms.** A character has one seat, in the room its
  session names.
