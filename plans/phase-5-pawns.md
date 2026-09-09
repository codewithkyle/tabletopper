# Phase 5: pawns

Read `plans/vtt-overview.md`, then `plans/phase-2-protocol-core.md` for the
pawn commands, the object kind and per-axis snapping, `plans/phase-3-transport.md`
for `Dispatch` and resolution, and `plans/phase-4-renderer.md` for the render
passes this adds to. This phase puts creatures and objects on the table:
spawning from the monster manual, the token library and the party's
characters; selecting one or many; moving them with snapping and the shared
movement path; and editing them through a dialog.

## As built

### Checkpoint 1, the server (2026-09-08)

Resolution, the write-through, the routes, the fragments and the window
plumbing shipped. Nine things differ from what the sections below sketch, each
for a reason the code carries in a comment:

- **Snapping modes are `off | cells | halfCells`, not "corners".** `snap.go`
  answers the parity question with cells mode and drops it in half-cells mode;
  there is no corners mode to port. `path.ts` is a port of `SnapAxis` as it is.
- **`pawn.spawn` gained a `size` field.** The wire carried no size at all, so a
  token could only ever land medium -- and size is the one stat that is
  structural rather than informational: it decides the footprint, which decides
  the snapping lattice and where a click actually puts the pawn. Hit points and
  armour class stay off the wire and are the pawn window's, per decision 10.
  Resolution reads the field; `Apply` is unchanged.
- **The room stat block needs no new statement.** `GetMonster` and
  `ListMonsterActions` both already take an `OwnerID` parameter, so "scoped to
  the room owner" is which id goes in rather than a second copy of the SQL.
  Decision 13's `GetMonsterStatBlockForRoom` would have been a byte-identical
  duplicate; the actions query, which decision 13 does not mention, would have
  needed one too and has the same answer.
- **The stat block is GM-only.** The route table said "any member when the pawn
  is visible", which contradicts the room's own monster-health setting: a stat
  block carries hit points, armour class and legendary actions, so serving one
  to a player undoes in one window what the GM chose in another.
- **The spawn dialog is two routes**, `/fragment/room/spawn` and
  `/fragment/room/spawn-list`, which is the shape the map picker and the asset
  manager already have. One route means a search swaps the box being typed into
  and the caret jumps to the end on every keystroke.
- **The conditions editor adds rows through a fragment**,
  `/fragment/room/condition-row`. `repeater.js` only REMOVES rows now; the
  character sheet adds them through `/fragment/character/feature-row`, and a row
  built in JavaScript would render unstyled because `public/js` is not a
  Tailwind source.
- **The write-through fires on every `pawn.updated` for a player pawn with a
  character**, rather than diffing hit points. Knowing that hit points changed
  means carrying the before-image through a path whose whole value is that an
  event carries the entity and nothing else; what it saves instead is a
  statement that writes the value already there, a few times a session.
- **`resolveParty` returns early for a player.** Resolution runs before
  Authorize -- the price of running it off the room's goroutine -- so a player
  pressing Spawn party was told "everybody already has a pawn", which is true,
  useless, and not why they were refused. Leaving `Pawns` nil hands the refusal
  to Authorize.
- **`hx-vals` on the DELETE arrives in the QUERY STRING.** This htmx tests
  `/GET|DELETE/` against the method and appends to the URL for both, while
  net/http's `ParseForm` reads a body only for POST, PUT and PATCH. `ParseForm`
  is exactly the union of the two; `r.PostForm` would have found nothing.

**The CSS diff caught one thing and it was the attribute vector.**
`list="condition-names"` -- HTML's own attribute, tying the condition field to
its `<datalist>` -- put DaisyUI's whole `.list` family in the build: fourteen
selectors and 2.4 KB. The word cannot be renamed, so the attribute is built in
`room-pawn.go` as a `templ.Attributes` and spread into the markup, which is the
same move the ban on prose makes for the same reason. Every other selector the
phase added is one the new markup uses.

**The menu items are not wired yet.** `Spawn pawns` and the player's
`Place my pawn` land with `dialogs.ts` in checkpoint 3, because an enabled menu
item whose click does nothing reads as broken -- which is the argument
`room.go` already makes for the disabled ones.

### Checkpoint 2, the rendering (2026-09-08)

The sprite cache, the three passes, the ruler's arithmetic, the layer filter and
the stress toggle shipped. Six things differ from what the sections below
sketch:

- **A sprite keeps its aspect ratio.** Decision 1 says "resized to 256 by 256",
  which is right for a monster picture and a portrait -- both are stored square
  already -- and wrong for the case the object kind exists for. A wagon is
  stored 512 by 171; squashed into a square and stretched across a two-by-four
  footprint it is visibly the wrong wagon. So the longest edge becomes 256, the
  layer is partly unwritten exactly as an edge tile's is, and the real pixels
  ride in the slot -- machinery the tile cache already had. Creatures then COVER
  their disc (cropped, because a portrait with bars down the side inside a
  circle looks like a mistake) and objects CONTAIN theirs.
- **`createImageBitmap` cannot fit, so the decode is two steps.** resizeWidth
  and resizeHeight are absolute: giving both distorts and giving one needs to
  know which edge is longer, which is what the first decode answers. The second
  call resizes an already-decoded bitmap, which is cheap and still off-thread.
- **The glyph atlas is a module of its own**, `render/glyphs.ts`. The plan asks
  for one without naming a file; it is fourteen characters rasterised once, and
  a label is an index into them rather than a canvas re-uploaded every time the
  dragged cell changes.
- **Hairlines are DEVICE pixels and text is CSS pixels.** A pawn border, a
  condition ring and its spacing are measured in the pixels that actually exist
  -- which is the argument the grid's own shader makes for drawing itself
  exactly one device pixel wide -- and the distance label is measured in the
  ones a person reads. Mixing them made rings twice as far apart as they were
  thick on a retina screen, which is how the two units got separated.
- **The sprite cache re-touches its own working set every frame.** The pawn
  buffer is rebuilt on a CHANGE and frames happen continuously, so a picture
  arriving several frames after the rebuild that asked for it would claim a
  layer while nothing had touched the ones on screen -- and with a full cache,
  claiming means evicting them. `begin(rebuilding)` is the fix: forget the set
  only when the caller is about to name a new one.
- **`render/scene.ts` holds the layer filter**, so rendering, hit testing, the
  marquee and the riders lookup all read one list. Phase 5's client module list
  has the filter as a property of four modules; one function is one place for it
  to be wrong.

**The shaders are checked with glslangValidator, not by looking at them.** All
five programs compile as GLSL ES 3.00 and LINK -- the second catches a varying
declared `flat` on one side and not the other, which is a link error and
produces a black table with no other symptom. The extractor lives in the
session scratchpad rather than the repo, because wiring it into `make check`
would make the build depend on a tool that is not currently required. It found
one real thing on the first run: `.replace("BORDER", ...)` on a shader source is
a runtime string operation nothing verifies, and a replace that matched nothing
would ship an uncompilable shader. It is a `#define` now.

**The CSS diff is empty.** The one markup change is the debug panel's Stress
button, which is `btn btn-xs` and already in the build.

**Nothing drives the path pass yet.** It is built, wired and drawn in the right
order -- under the pawns for the highlighted cells, over them for the line and
the label -- and the drag that fills it is checkpoint 3.

### Checkpoint 3, the interaction (2026-09-08)

Selection, the drag state machine, riders, placement and the overlay shipped,
and the two menu items that reach them went live with them. Seven things differ
from what the sections below sketch:

- **A tool gets first refusal on the primary button, and hears every press
  either way.** `input.ts` grew a `Tool` contract: `press` answers whether the
  CAMERA should keep out of the gesture, and the tool is told about the press,
  the drag and the release whichever way it answered. That second half is what
  lets a click on empty table clear a selection without taking panning away from
  every empty part of the table.
- **There is no Select tool.** Move owns it, per the decision taken before this
  phase started: press a movable pawn to drag it, press empty floor to pan,
  Shift-drag to marquee. The pill's other three tools do not gate the table yet
  and phase 6 wires all four at once -- gating on features that do not exist
  would mean a GM who pressed Measure found a table where nothing worked.
- **Other players' drags are told apart by COLOUR and not by a name label.** The
  plan asks for "a small name label on the anchor", and the glyph atlas is
  deliberately fourteen characters -- the digits, a space, an f, a t and a full
  stop. Rendering arbitrary player names means a full Unicode atlas, which is a
  different feature; a fixed palette indexed by a hash of the player id
  distinguishes six people at a table and costs nothing.
- **`hx-vals` sends the selection as ONE comma-separated value.** htmx SETS each
  key rather than appending it, so an array arrives joined and there is no way
  to make it emit a repeated field. `pawnIDs` splits on commas as well as taking
  repeats; a ULID has no comma in it, so it is exact.
- **The overlay's condition dots are cloned from a `<template>`.** A colour is a
  class name and `server/js` is not a Tailwind source, so the eight dots are
  rendered in templ and the script clones the one it wants -- the same move the
  `list` attribute made in checkpoint 1, for the same reason.
- **The page carries `data-user`.** The canvas needs the viewer's own id to
  answer "may I move this". It is not a secret from the table -- the player list
  already carries every member's id for the kick button -- and it is not
  authority either: every command is authorised server-side, so a browser that
  lied about it would build a selection whose every move came back forbidden.
- **A player's `Place my pawn` arms rather than opening a dialog**, and raises
  the same `room:arm` event the spawn dialog does. There is nothing to choose:
  they joined with one character and the only question is where. *(Reverted in
  the rework below: players no longer place pawns at all.)*

**Three integration bugs, all found by wiring rather than by tests.** Moving the
hand from a pawn toward its own Details button fires `pointerleave` on the
canvas, which cleared the hover, which hid the overlay out from under the hand
reaching for it -- so the question asked is now whether the pointer left the
MOUNT. The overlay's text was set only when the selection changed, so a goblin
taking damage left it stale. And `expirePreviews` was written and never called,
which would have left a ghost from a tab that closed mid-drag on every client
for the rest of the session -- and, because a live preview keeps the frame loop
awake, every client rendering for ever to draw it.

**The wagon test was wrong and the code was right.** Pressing at a wagon's
centre grabs the RIDER standing there, because the topmost pawn is what a click
means and a passenger is above what carries it by definition. The tests now
press an empty corner, and there is a third that asserts the rule that made the
first two wrong.

**Four new CSS selectors**, all in the overlay: `empty:hidden` on the conditions
row, `max-w-64`, `pointer-events-auto` on its two interactive blocks -- the root
is `pointer-events-none` so it does not take clicks from the table underneath --
and `will-change-transform`.

### Rework 1, spawning is the GM's (2026-09-08)

The first pass gave a player a live `Place my pawn` and made the GM's `Spawn
pawns` open a dialog. Both were wrong and both are gone.

- **`Spawn pawns` posts and opens nothing.** It is `POST
  /rooms/{id}/pawns/party`, which is the `pawn.spawnCharacters` command that
  already existed: a pawn at the centre of the map for everybody connected who
  joined with a character and has none yet. There was never anything to ask --
  the roster and which characters are already down are both room state, read by
  the hub when it resolves -- so the dialog it used to open was four clicks
  around a question with one answer. The GM drags them where the party actually
  is, which is one gesture.
- **The library dialog kept its own item, `Spawn from library`.** Monsters and
  tokens still need somewhere to come from, and that half of the dialog was
  working; renaming its entry rather than deleting it leaves the decision about
  where it eventually lives open. Its own `Spawn party` button is gone, because
  the menu item now owns that act and one act with two buttons is one that will
  drift.
- **A player's Tabletop menu is five disabled lines.** Putting something on the
  table is the GM's act, and nothing under Tabletop is a player's.
- **`PawnSpawn.Authorize` is GM-only, which is the half that matters.** The
  allowance for a player's own character was REMOVED rather than merely hidden:
  the socket takes commands from whoever is connected, so a rule that only a
  missing button enforces is not a rule. `requirePlayerLayer` went with it (a
  no-op for the GM), as did `Apply`'s "a player's pawn belongs to whoever placed
  it" branch, which nothing can now reach.
- **Resolution bails out for a non-GM before it touches the database.**
  Resolution runs BEFORE authorization -- the price of running it off the room's
  goroutine -- so without the early return a forged spawn off a player's socket
  would read a manual on behalf of a command about to be refused.
  `resolveCharacter` lost its whole player branch with it.
- **`room:arm` is gone as a window event.** It existed because the menu bar in
  `public/js/room.js` armed the canvas and could not import `dialogs.ts`. With
  that item gone the only thing that arms is the dialog, which is in the same
  bundle as the canvas, so arming is a direct call and the event is not left
  behind as a hook nothing pulls. `RoomPageData.CharacterID` went with it.

The scenario fixture changed by three lines: the GM places Ari's character
instead of Ari, so the `by` on that `pawn.spawned` is the GM's id and two step
names read differently.

### Rework 2, the picture decides, and the table can be cleared (2026-09-08)

Five items, of which two are the same change: a token stops being asked about.

- **A window's title bar can shrink.** A long pawn name pushed the minimize,
  maximize and close controls past the window's right edge, where the section's
  `overflow-hidden` cut them off and the window could not be closed. The bar is
  a row of a grid whose column is sized `auto`, so the column's floor was the
  bar's own min-content width -- and the heading's `white-space: nowrap`, which
  is half of `truncate`, made that the whole title. `min-w-0` on the bar makes
  its automatic minimum size zero and the heading truncates as intended.
  Reproduced and confirmed in a headless browser before and after.
- **The token half of the spawn dialog asks nothing.** The Creature-or-Object
  switch is gone -- a token is an object, a picture of a thing on the table, and
  a creature is a monster out of the manual or a player's character -- and so
  are the creature-size select and the two cell-count inputs. What is left is a
  search, a Players-see-it switch and a wall of pictures.
- **An object is drawn at the size of its picture, in map pixels.**
  `Pawn.FootprintW`/`FootprintH`, which were CELLS, became `Pawn.Width`/`Height`
  in MAP PIXELS; `Pawn.Footprint` now takes the cell size and rounds, and is the
  snapping lattice rather than the drawn size. `PawnSpawn` lost its size fields
  entirely: the assets row holds the picture's dimensions, so the hub reads them
  in `resolveObject` and there is nothing for a browser to say about it. The
  pawn's own dialog still lets the GM override it, now in pixels. `Schema` went
  to 2, because a 2 that meant two cells would decode as two pixels -- an
  invisible wagon rather than a decode error.
- **The stat block in a window has no `Close`.** The one at the bottom fired
  `modal:close` at a modal that was not open. `MonsterStatBlockPanel` is the
  dialog's frame minus its footer; the modal keeps its `Close`, because a modal
  must ship a labelled way out and a window is dismissed by its own title bar.
- **`Clear tabletop` works.** `table.clear` empties every layer's map, every
  pawn, all the fog, all the drawing and the tracker, in one command. The floors
  themselves stay -- a room needs at least one -- and so does the grid. It emits
  one `pawn.removed` per pawn rather than an event of its own, which is what
  removing a populated layer already sends, so every open panel already does the
  right thing with it. It is the only item in the Tabletop menu behind a
  confirmation, and the confirmation names what goes.

### Rework 3, tokens lie on the floor and can be turned (2026-09-08)

Four items, of which three are one idea: a token is a picture laid on a map, not
a creature standing in a square.

- **Every token is drawn under every creature, and a click follows the draw
  order.** `compareStack` in `render/scene.ts` is one comparison read by the pawn
  pass, the hit test and the riders lookup: object first, then `z`, then id.
  Objects are the floor's furniture -- a rug, a road, a bloodstain, a wagon --
  and a party that walked onto a rug spawned after them used to vanish under it.
  `z` still orders within a kind, so two rugs stack in the order they were laid.
  The hit test uses the same comparison rather than `z` alone, so clicking where
  a goblin stands on a rug picks the goblin; the rug is reachable everywhere the
  goblin is not, and the map is what is under both.
- **Right-clicking abandons the gesture in hand.** `Tool.secondary` is Escape for
  a hand that is already on the mouse: it disarms placement, puts a dragged pawn
  back, or drops a marquee, and answers whether it did any of those. Only when it
  did is the browser's own context menu suppressed, so a right click on empty
  table is still a right click on a web page. *(Reworked below: the menu is
  suppressed unconditionally, and a right click with nothing to abandon asks
  about the pawn under it -- by opening its window as of Rework 4, and by
  putting up a menu of its own as of Rework 11.)*
- **A token is never snapped.** `snapPawn` returns an object's centre untouched
  on both sides -- `internal/room/snap.go` and its port in `render/path.ts` -- so
  a rug, a door, a road sign and a wagon are placed against what the cartographer
  drew rather than against the lattice over it. `Pawn.Footprint` lost its object
  branch and its cell-size argument with it; the client's `footprintOf` is now
  `Size.Footprint` exactly, and the riders gate that used to ask for it measures
  two cells in pixels instead. The drag's clock followed: with nothing to snap to
  there is no cell boundary to report on, so a token's drag reports on the timer
  that snapping-off already used.
- **A token can be resized and turned.** `Pawn.Rotation` is whole degrees
  clockwise about the centre, an object's alone, folded into `[0, 360)` by
  `normalizeRotation` wherever it is written -- an angle is a wrapping quantity,
  so -30 is reduced rather than refused. It is not on `PawnSpawn`: a token goes
  down square and is turned afterwards, by the eight boxes and one circle in
  `js/room/handles.ts` or by the Angle field in the pawn window, both of which are
  `PawnUpdate`.

  **Everything scales about the centre**, resizing included, and that is the one
  choice here worth arguing. An image editor anchors the opposite corner, which
  makes a resize a change of size AND position -- and position is `PawnMove`'s,
  snapped and authorised separately, so one gesture would send two commands that
  can be refused independently and a token could end up somewhere nobody dragged
  it. Scaling about the centre keeps the whole gesture inside one idempotent
  command and gives the two gestures one origin instead of two.

  Shift locks a corner's aspect ratio and steps a rotation by 15 degrees, which
  is the only way to get a token exactly square again once it has been turned by
  hand. The handles are a fixed size on screen and a moving size on the table,
  which is the one number `TableDeps.scale` exists for.

  **The rotate handle hangs below the token rather than above it**, which is
  where every other editor puts one. Above is already taken: the pawn overlay is
  placed on the top of the same box and lifted eight pixels off it, and the
  handles are only ever up when exactly one pawn is selected -- which is exactly
  when that overlay is showing. A handle up there is not occasionally covered,
  it is always covered. So the angle is measured from straight DOWN, and a
  quarter turn clockwise is the hand dragging to the left. *(Rework 5 retired
  that collision -- a token is never labelled now -- but the handle stays below,
  on the first half of the argument.)*

  The rotation reaches the GPU as a cosine and a sine per instance in both the
  pawn pass and the ring pass; the fragment shaders never learn the angle,
  because their arithmetic is already in the quad's own local space. The hit test
  turns the POINT into the token's frame rather than growing a box round it, and
  `boundsOf` is the screen-aligned box the DOM overlay is placed on, which for a
  long token turned on its side is `pawnExtents` the other way round.

  A resize or a rotation is previewed locally as a ghost and committed as one
  `pawn.update` on release. Nobody else watches it happen: `pawn.dragging` carries
  positions and only positions, and a second hot-path event for a gesture that
  lasts a second and happens between fights is not worth the protocol.

### Rework 4, the pawn window is the pawn's whole surface (2026-09-08)

Eight items, of which six are the same move: everything that was behind an Edit
button in a modal is now in the window a right click opens. *(Rework 11 moved
the opening gesture to a double left click; the right button puts up a menu
whose first item opens the same window.)*

- **The canvas has no browser menu.** `onContextMenu` in `render/input.ts`
  prevents the default unconditionally and before the tool is asked, which
  replaces Rework 3's rule that it was suppressed only when something was
  abandoned. A right click on a tabletop is a gesture in the application: it puts
  down what the hand is holding, or it opens the thing under the pointer.
  Offering "Save image as" on one part of the table and a gesture on another
  makes one button do two unrelated things depending on where it lands.
  Everything else on the page keeps its menu for free, because every listener in
  that file is on the canvas -- a right click on a window, a field or the menu
  bar never reaches it.
- **Right-clicking a pawn opens its window.** `Tool.secondary` takes the point
  now and answers nothing; it abandons FIRST and asks about what is under the
  pointer only when there was nothing to abandon, because a GM halfway through
  placing an encounter pressed it to stop. It uses the same `hitTest` as a left
  click, so it picks the goblin standing on the rug rather than the rug, and it
  does not touch the selection: answering "let me look at that" by throwing away
  what somebody had picked out would make it a destructive gesture. *(Rework 11:
  what it asks for is a menu. Everything else in this item still holds.)*
- **What a pawn's window IS lives in `js/room/pawn-window.ts`.** Four things
  reach for it -- the overlay's Details button, a right click, the retitle that
  follows a rename and the close that follows a removal -- and two of those aim
  an id at a window somebody else opened. An id spelled differently in any of
  the four is a window that opens twice and never closes.
- **The pawn edit modal is gone**, along with `/fragment/room/pawn/edit`,
  `RoomPawnFormData` and `RoomPawnForm`. Name, creature size or pixel size and
  angle, maximum hit points, armour class, conditions, visibility and floor are
  all in the panel, as one form beside the quick hit-point control. `UpdatePawn`
  answers with the panel it just changed instead of closing a dialog, which is
  the shape `UpdatePawnHP` already had. (Rework 6 moved the name back into a
  dialog of its own, moved the maximum into the hit-point row, and made both
  saves answer with the error slot instead of the panel.)

  **The objection to a form in a refetching panel is answered by the panel's own
  trigger.** A swap while somebody is typing throws away what they typed, which
  is exactly why the form was somewhere that never refetched -- but the panel
  already declined every refetch while focus was inside it, to protect the one
  field it had. That guard covers the whole form unchanged. It is the same rule
  doing more work rather than a new rule.

  Two panels are open at once as a matter of course, so every id in the fragment
  carries the pawn's own: the element, the error slot, the form, the conditions
  list, the `<datalist>` and every labelled control. A shared id is a label that
  focuses the other window's field and an Add condition that appends the row to
  whichever panel happens to be first in the document.

  It also fixed a silent bug: `pawnPanel` read the room and the pawn out of the
  QUERY STRING, so the hit-point POST -- which carries them in the path -- was
  answering 404 to a request that had in fact succeeded. Nothing showed, because
  the page's noSwap config swallows a 4xx and the socket event brought the panel
  back into step a moment later. The two ids are parameters now.
- **Delete removes the selection, through the confirm modal.** `pawns.ts` hears
  the key and calls `TableDeps.remove`, which presses a button: `hx-confirm`
  lives on the element that makes the request, so pressing that element is the
  only way to get the dialog, and `window.confirm` is banned. *(Reworked below:
  that button is a hidden GM-only control on the room page rather than anything
  in the selection overlay, which has no buttons on it at all.)*

  The keys are heard on the document and the table is not the only thing on it,
  so a key that arrives from an input, a textarea, a select or anything
  contenteditable is not a key pressed on the table. A GM typing a goblin's new
  name into its window is rubbing out a letter, not a goblin. There is no role
  test: a player's page renders no such button, so there is nothing to press.

### Rework 5, the label is a label (2026-09-08)

Three items, all of them the same correction: the thing that follows the
pointer is a label, not a toolbar and not a panel.

- **The single-pawn label carries a name, hit points and armour class, and
  nothing else.** Details, Stat block and Remove are gone from it, and so are
  the condition dots. Everything about a pawn is in its window, which a right
  click opens; a row of buttons on something that follows the pointer is a row
  of buttons that moves out from under the hand reaching for it. `OverlayDeps`
  lost `roomID`, `role` and `user` with them -- all three were there to decide
  which buttons to draw.
- **A selected pawn is not labelled.** `Table.focus` is the HOVERED pawn now and
  never a selected one, and `Table.bounds` follows the same rule so the label
  cannot land somewhere other than the thing it is labelling. What a hand does
  next to one selected pawn is drag it, turn it or resize it, and all three
  happen exactly where the label was sitting. A multiple selection is the one
  case that stays up, unchanged, because it is not describing a pawn at all: it
  is the count, the Move to floor select and the group's Remove button.
- **A token is never labelled at all.** No hit points, no armour class, and a
  name the picture already tells you -- and the space over a token is where its
  eight resize boxes and its rotate handle are. This is also what retires the
  reason the rotate handle went below the token in Rework 3; it stays there,
  because below is where every other editor puts one.
- **The Delete key presses a hidden button on the room page**, `data-pawn-remove`,
  rendered for the GM alone and kept in step with the selection by the overlay
  on every refresh. A keyboard shortcut needs an element to press -- `hx-confirm`
  only works on the element making the request -- but it does not need one
  anybody can see, and this is what lets the label have no buttons on it while
  Delete goes on working for a selection of one.

### Rework 6, the pawn window is a panel and not a form (2026-09-08)

Five items, and the last of them is why the first four are safe.

- **The hit-point row is `HP [current] / [maximum]`.** The maximum was a field
  of its own halfway down the editor, which is not where anybody looks for it:
  the two numbers are one reading -- "4 / 7" is how a table says it -- and a row
  that shows one of them is a row you have to go and check. They are one form on
  the hit-point route now, and both go in one `pawn.update`, which is also what
  makes raising a maximum and healing to it a single entry: `PawnUpdate` clamps
  once, after applying everything it was given.
- **Size, with armour class beside it when the window is wide enough.** A
  container query rather than a viewport one: what decides whether two fields fit
  is how far the GM dragged the window's edge, and the panel already declares
  `@container` for the readings. An object gets width and height in one row and
  angle and armour class in the next, on the same rule.
- **The name is a heading with a Rename button beside it.** It was a text field
  taking a row of a 320 pixel window for a value that changes once a session and
  is read every second of it -- and, once the panel autosaved, a field that would
  rename a pawn on every pause mid-word: four renames and four events to reach
  "Goblin archer". It opens the content modal on a one-field fragment, prefilled,
  which posts to a route of its own because the editor's POST replaces the pawn's
  whole condition list from the rows its form carried.
- **The three buttons are one row in the header**: Stat block with a label, and
  Rename and Remove as icons beside it. Remove was at the bottom of the panel,
  which put the destructive control below the fold of the default window; its
  confirm is unchanged. *(Rework 8: all three are icons, and a character has no
  Rename.)*
- **There is no Save button, and that is the change the rest depended on.** The
  old one sat below every other control, which in a 320 by 520 window meant below
  the fold: the fields looked broken because the thing that committed them could
  not be seen. Both forms autosave now -- the editor on `input delay:400ms` plus
  the `repeater:changed` a deleted condition row raises, the hit-point boxes on
  `change` -- and this is the character sheet's own `savingPanel` shape, down to
  the 422 and the error slot.

  **Autosaving is only safe because neither save swaps the panel.** A reply
  carrying the panel arrives while somebody is still working in the window and
  replaces the field they have moved on to; both routes answer with the panel's
  error slot instead, empty on success, and the socket event is what brings every
  open copy back into step. That in turn is why the refetch filter changed from
  "focus is anywhere in this panel" to "a box in this panel has the caret", which
  it asks by `activeElement.type` being `text` or `number`: a select or a
  checkbox has nothing half-entered to lose, and those are exactly the controls
  whose own save has to bring the panel back -- ticking "players can see this
  pawn" changes the Hidden badge in the header.

  **A square bracket and a comma are both forbidden inside an `hx-trigger`
  filter**, which is why that check is not the CSS selector it obviously wants to
  be. The filter is delimited by the brackets around it and the attribute is
  split on commas, so `'input[type=text],textarea'` ends the filter early and
  leaves the rest parsed as trigger modifiers -- silently: nothing errors, the
  panel just stops refetching.
- **Both hit-point boxes take a sum: `23-7`, `23-7-4`, or a bare `-7` counting
  from what is there.** The server evaluated one signed change already; it
  evaluates a chain now, and `js/room/hp.ts` evaluates the same strings in the
  browser so the number appears the moment the box is left. That client half is
  what lets the row autosave without a reply to swap in, and it is the same
  arrangement `path.ts` has with `snap.go` -- two implementations of one rule,
  with a test either side. It listens in the CAPTURE phase, because htmx's own
  `change` handler is on the form above it and would otherwise read the box
  before the sum was resolved.

### Rework 7, the label is one setting and the setting is the whole label (2026-09-08)

`Table.MonsterHP` is `Table.PawnLabels`, `hidden | band | exact` is
`none | default | full`, and what it governs is no longer just hit points.

- **Three points on one scale, and the scale is what the table is told.**
  `none` labels nothing for anybody, including the GM. `default` is the GM's
  table: the GM reads numbers, the party reads words. `full` is an open table
  where everybody reads the same numbers. The fourth combination -- the GM told
  less than the players -- is not a way anybody runs a game, which is why this
  is one control and not a pair of them.
- **Armour class travels with the hit points now.** Telling the party what to
  roll against is the same fight-solving arithmetic the words exist to avoid, so
  `projectPawn` strips `AC` from a monster or npc on `none` and on `default`, and
  keeps it only on `full`. It is stripped in the PROJECTION and not in the
  label, which is the whole difference between a secret and a hidden element:
  what is not projected is not in the browser to be read out of.
- **Six bands instead of four.** Healthy above three quarters, bruised down to
  a half, bloody down to a quarter, very bloody down to a twentieth, near death
  below that, and dead at zero. The scale is coarse at the top because the
  answer above three quarters is always "it is fine", and fine at the bottom
  because that is the only part of it anybody is making a decision on. Dead
  stays separate from near death: a creature at zero is down, and a creature at
  one is the reason somebody spends their turn attacking rather than running.
  `pages.PawnBandText` and `js/room/overlay.ts` each print the six, and a test
  either side pins the pair.
- **`none` is the one setting the client reads.** The other two are carried
  entirely by what `Project` leaves on a pawn -- a player in a `default` room
  was sent a word and no numbers -- so the label prints whatever arrived and has
  no idea which room it is in. `none` cannot work that way, because it takes the
  label away from the GM too and a GM's copy is never projected: there is
  nothing missing from it to notice. That single read is `OverlayDeps.labels`.
  It does not take the GROUP panel away, which is not a label at all but the
  count and the GM's two controls for acting on a selection.
- **An old snapshot is repaired rather than discarded.** A room saved as
  `monsterHp: "band"` reads back as a `pawnLabels` nothing accepts, and
  `Normalize` sets it to `default` -- band's successor, and what the great
  majority of those rooms were on. A room that had been set to either extreme
  comes back in the middle once. The alternative is bumping `Schema`, which
  throws every pawn on every table away to save the GM one click.

### Rework 8, the window panels are compact and the buttons are icons (2026-09-08)

The pawn window was built with the character sheet's field components, which are
sized for a full page. In a 260-pixel window they are the wrong shape.

- **Every control in the panel is the extra-small one**, which is what the
  tabletop settings window already used. `fieldset` and `fieldset-legend` are
  gone from it: between them they cost about a rem of vertical padding per
  field, for a legend that is a ten-pixel caption here. What replaced them is
  the settings window's own shape -- a `label` holding a caption span and the
  control -- and `TestEveryControlInThePanelIsCompact` fails on an `input-sm`
  or a `fieldset-legend` reappearing.
- **The empty error slot is `hidden`.** It was a zero-height `div` between the
  header and the hit-point row, and its parents are flex columns with a gap --
  so an element with nothing in it was taking a gap on each side of itself. The
  reasoning now lives in `templ/pages/panel-form-errors.go`, which is also the
  home the component never had.
- **The header's three buttons are icons with tooltips.** Stat block was the
  last worded button and it is the tabler book; Rename is the tabler label;
  Remove is the tabler outline trash, which also replaces the layer manager's
  filled one and the condition row's kick icon. A worded button on that line
  pushes the pawn's own name out of view at 260 pixels.
- **Every icon button in every room window carries a tooltip AND an
  aria-label**, and the two are different strings deliberately: the tip is read
  beside the thing it points at, so "Remove" is unambiguous, while a screen
  reader announces the button with no such context and eight goblins would be
  eight buttons announcing "Remove". That covers the pawn window's four, the
  layer manager's three, the member list's kick and the window chrome's
  `─ □ ✕`. The tips point INTO the panel -- `tooltip-bottom` in a title bar,
  `tooltip-left` at a right-hand edge -- because a window is `overflow-hidden`
  and a tip pointing out of one is a tip that is clipped.
- **A player's character has no Rename button.** The name came off the sheet its
  player wrote, every other player reads it in the turn order, and the one
  plausible reason to change it here is a typo -- which is a thing to fix on the
  sheet, where it will still be right next session. `RoomPawn.Character` is the
  pawn kind narrowed to that one question.
- **Visibility is a switch that says which state it is in.** "Players can see
  this pawn" beside a checkbox made the reader work out what the position of the
  box meant; the switch now has "Hidden" or "Visible" beside it, swapped by
  `peer-checked` so the word is right the instant it is clicked rather than a
  round trip later.

### Rework 9, a window scrolls down and never across (2026-09-08)

- **The window body is `overflow-y-auto` with `overflow-x` hidden.** A panel in
  a window is a column of controls whose width the reader chose by dragging an
  edge, so a horizontal scrollbar is never the answer to anything: it is how a
  panel that failed to reflow reports itself. Clipping turns that into a visible
  bug in the panel rather than a bar somebody has to use.
- **Every tooltip in a window is `tooltip-left`.** DaisyUI positions a tip
  absolutely inside the element it belongs to and leaves it in the layout at
  zero opacity, so a tip centred over a button at the panel's right edge
  overhangs that edge whether or not anybody is hovering -- and inside a scroll
  container an overhang IS horizontal overflow. That was Rework 8's own
  `tooltip-bottom` header row, and it is the reason the pawn window grew a
  scrollbar on every kind of pawn at once. Pointing left puts the whole tip over
  the panel, where there is always room.
- **The condition row stacks.** Five controls -- name, colour, turns, what it
  counts down against, remove -- need about 260 pixels of fixed width between
  them before the name has anywhere to go, which is more than a 320 pixel window
  has once its padding and its scrollbar are off. The name is on its own line
  now with the four small controls under it, back to one line at a container
  width where the name is still readable, and every control in it can shrink so
  the 200 pixel minimum window clips nothing.
- **The floor select and visibility are the first row of the editor**, in that
  order, not the last. They are the two controls a GM reaches for mid-fight and
  everything else in that form is set once; under the condition rows and the Add
  condition button they were below the fold of the default window. The select
  takes the row and the switch sits at its end, because a floor name is as long
  as the GM made it and a switch is two words at most. They are not in the
  header beside the icon buttons, which is where they belong visually, for a
  mechanical reason: both are form controls the editor's POST has to carry, its
  trigger listens on its own form, and a control outside a form raises no events
  into it. The floor select is unchanged otherwise -- it has always been the
  GM's and has always been drawn only when the room has more than one floor,
  which is why a one-floor room shows no such control.

### Rework 10, the colour picker has an alpha slider (2026-09-08)

- **The grid colour is picked with `<hex-alpha-color-picker>` and not with
  `<input type="color">`.** The native input has no opinion about opacity, and
  the grid is drawn over somebody's map -- the alpha pair is the half of the
  value that actually gets tuned, and it was left to be typed and guessed at
  two hex digits at a time. The `alpha` attribute answers that on Chromium and
  Safari, and a browser without it ignores the attribute silently, which is the
  same guessing with a worse explanation.
- **The picker is vanilla-colorful, pinned in package.json and bundled into
  room.js**: 7KB minified, 3KB over the wire, MIT, no dependencies. It was
  chosen over Coloris, Pickr and iro.js for one property that matters here more
  than the size: it is a custom element and every style it has is inside its
  own shadow root. There is no stylesheet to add to public/css, nothing for
  Tailwind to scan, no second theme to keep honest against caramellatte and
  coffee, and no class name in server/js -- which is banned anyway, because
  server/js is not a Tailwind source and a class name written there is never
  emitted.
- **The text field is still the form field.** It keeps `name="color"`, its
  pattern and its value; the picker writes into it and the form posts what it
  always posted. Nothing on the server has heard of the picker, `checkColor`
  already took 6 or 8 digits, and a browser running no scripts gets an
  unupgraded tag it ignores beside a colour it can still edit by hand.
- **The picker is folded away behind the swatch.** The window is 300 by 420 and
  its contents already scroll; 176 pixels of permanent picker would push half
  the settings below the fold for a setting a GM changes once a campaign. The
  swatch is the disclosure and carries `aria-expanded` and `aria-controls`, and
  what the script toggles is `hidden` -- the only thing it can toggle.
- **The template paints the swatch and the client repaints it.** The panel is a
  fragment htmx swaps into a window with no script of its own to run, so a chip
  left to the client would be blank until the GM touched something. `SwatchStyle`
  is an inline `background-color` over the white the button puts behind it, so a
  colour at a fifth opacity looks like a fifth of itself and not like a darker
  panel.
- **The form is told once the drag has settled.** `color-changed` fires on every
  pointer move and the form saves on `change`, so forwarding each one would be a
  POST per frame. The field's value is written live -- the hex and the swatch
  track the drag -- and the `change` htmx listens for is held until the picker
  has been quiet for 300ms.
- **Every accepted spelling becomes `#RRGGBBAA` in upper case.** vanilla-colorful
  drops the alpha pair when a colour is opaque and answers in lower case, so
  without normalising, a drag across full opacity would flip the box between
  `#ff0000` and `#ff0000cc` -- two spellings of the same setting, in a box whose
  whole job is to show what the alpha currently is.

### Rework 11, a double click opens a pawn and the right button asks (2026-09-08)

- **A double left click on a pawn opens its window, and the right button no
  longer does.** This came out of playing at a real table rather than out of
  review: people reached for a double click without being told to, found
  nothing under it, and never discovered the right click that worked. The
  gesture people look for is the one the app should have.
- **The pair is counted in `pawns.ts` and not taken from the browser's
  `dblclick`.** Every primary pointerdown on the canvas is `preventDefault`-ed
  -- without it the middle button's scroll puck appears over the map and stays
  there -- and a prevented pointerdown suppresses the compatibility mouse
  events a `dblclick` is assembled from, which browsers do not agree about.
  What is given up is the platform's own double click speed, which a page
  cannot read anyway; `DOUBLE_MS` is 400, a little under the half second the
  platforms default to.
- **A gesture that was not a click breaks the pair.** `release` asks
  `clickedPawn` once, for every kind of gesture, and a drag, a resize and a
  marquee all answer nothing -- so click, quick drag, click is three things
  that happened rather than one gesture, and the branch handling each does not
  have to remember to say so.
- **`Panning` now carries an anchor**, which is what makes the gesture work for
  a player. A monster is not theirs to drag, so their press is handed to the
  camera and never becomes a `Pressing` -- and without the anchor the viewer
  most likely to be asking "what is that" is the one it would not work for.
- **Shift is left out of it.** Two shift clicks on one pawn put it into a
  selection and take it straight back out, which somebody does on purpose; a
  window landing on the table halfway through picking a group is not.
- **The right button puts up a context menu**, at the pointer, about the one
  pawn under it. Open details for everybody; Move to floor and Remove pawn for
  the GM. The two GM items previously had nowhere to be asked for: the
  overlay's floor select and Remove appear only for a selection of several, and
  the Delete key acts on the selection and needs a keyboard.
- **It acts on the pawn under the pointer and never on the selection**, which
  is what the right click asked about, and is why the removal's confirmation
  names the pawn.
- **The two mutations are ordinary htmx buttons that this only writes
  `hx-vals` onto and presses.** Removing pawns is confirmed, the confirmation
  is the app's confirm modal, and `hx-confirm` is read off the element making
  the request -- so a DELETE built in the client would skip the dialog, and
  `window.confirm` is banned. The floor move goes the same way and posts to the
  same route the overlay's whole-selection control does: one route, a list of
  one.
- **The floor rows are a `<template>` cloned per layer**, for the reason a
  window's chrome is one: `server/js` is not a Tailwind source, so a class
  named in `pawn-menu.ts` would never be emitted. The row is styled in templ
  and the client writes only text, a data attribute and `[hidden]` into it --
  including the badge that says which floor the pawn is already on, without
  which the list is five names with nothing to tell them apart.
- **The menu is stacked with `nextZ()`, exported from `window.ts`.** A popup
  given a fixed z-index in its own markup sits above the windows until somebody
  has raised twenty of them, and then silently goes under one. One counter for
  the table has no such hour.
- **It is placed again when the floor submenu unfolds.** Opening the list
  changes the menu's height, and a menu turned back at the bottom edge has to
  be turned back by more.
- **Fixed on the way past: the overlay's group "Move to floor" select had no
  `name`.** htmx collects values from named fields, so the layer never left the
  browser and the route answered 404 with an empty body -- a control that
  looked like it worked and did nothing.
- **A long name is trimmed rather than left to set the menu's width**
  (found in play, 2026-09-08). DaisyUI makes every row a flex item of a
  wrapping column, so a row with no width of its own is as wide as its content
  -- and the heading, set to `nowrap`, has a minimum width of the whole name.
  That row decided the width of the flex line, every other row was stretched to
  match it, and the hover backgrounds, the floor list and the "Here" badge ran
  out past the panel's border onto open table: measured in a browser against
  the built stylesheet, a 53-character name drew 426px rows inside a 224px
  menu. The heading now carries `block w-full truncate` -- `w-full` is the
  width the name cannot argue with, and `block` is what makes `truncate` mean
  what it says, because `text-overflow` belongs to a block container and the
  flex box a row is otherwise given would cut the name off mid-letter with no
  ellipsis. The floor rows were already safe: `min-w-0` on the name span is
  what caps them.

### Rework 12, the pill has a select tool and the space bar pans (2026-09-08)

- **Select is a real tool and it is what a room opens in.** The pill kept its
  own `aria-pressed` in `server/public/js/room.js` and the canvas never asked
  which button was pressed; two of the five modes are real now, so the state
  moved into the bundle the canvas is in (`server/js/room/tools.ts`) and the
  stub in `public/js` is gone. A control and the thing it controls in two
  scripts that cannot import each other is a contract with nobody to enforce it.
- **The group selection is a plain drag.** It used to need Shift, which is a key
  nobody finds on their own. A press on empty table -- or on a pawn this viewer
  may not move -- is now a click until the hand travels four device pixels and a
  marquee after that, which is the same threshold a press on a goblin uses to
  become a drag. Shift still means what it means everywhere else: a marquee held
  with it adds to the selection instead of replacing it, and a shift click that
  caught nothing leaves the selection alone.
- **`Panning` and `Marqueeing` merged into one gesture.** They were the same
  press described twice -- the camera's, watched for the click that clears the
  selection, and the rubber band -- and which one it was is only known when the
  button comes back up. The merged `Marqueeing` keeps the anchor that makes a
  double click work for a player who may not drag what they are asking about.
- **Move is now the camera's mode and nothing else's.** `deps.panning()` is
  asked first in `press`, before placement and before the hit test, and it
  records no gesture at all: no drag, no marquee, no selection, no spawn. What
  it deliberately leaves alone is the selection, because shoving the map across
  is not a reason to throw away the group somebody spent a minute building.
- **The mode is read at the press and never again during a gesture.** The space
  bar is a key a hand lets go of, and letting go of it halfway through a marquee
  must not turn the box into a pan.
- **The space bar is a held switch to Move**, which is the gesture every drawing
  program has trained every hand to expect. It is taken everywhere except a text
  field -- `keys.ts` is the one answer to "is this key the page's", now that
  `pawns.ts` and `tools.ts` both ask -- and taking it means the browser does not
  get it, because a focused button would read it as a press. Enter is what a
  keyboard is left with for buttons. A blur releases the hold, which is the one
  way it could get stuck. The canvas cursor goes to `grab` while it is down:
  the pill is in the corner and a GM panning is looking at the map.
- **Which tool pans is rendered rather than spelled again in TypeScript.**
  `room.go` puts `data-room-tool-pans` on exactly one button and `tools.ts`
  finds it by that attribute -- a name written out in both languages is a space
  bar that quietly stops working the day the list is renamed. The mode a room
  opens in is read the same way, off the button rendered pressed.
- **Measure, Fog and Draw leave the table on Select.** Their features are not
  built, and gating the pointer on them would drop a GM into a mode where
  nothing works and nothing says why. Phase 6 builds the three.
- **A floors menu in the pill swaps the ACTIVE layer.** The bar picks the floor
  this GM is looking at and the layers window is where floors are made; neither
  is the thing a GM does over and over while running a fight, which is putting
  the party on the roof and letting the players see the roof. It is the same
  POST the radio in that window makes, so there is one route and one event, and
  the GM's own view follows because activating a layer clears the local
  override. It is the GM's or it is nothing.
- **The menu is a sibling of the table rather than a child of the pill.** The
  pill is positioned and z-indexed, so it is a stacking context, and a dropdown
  inside it could never rise above a window sitting under that corner -- and a
  DaisyUI dropdown inside the tooltip wrapper every other button has would put a
  tooltip over its own list. It is placed against the button by
  `layer-tool.ts` and raised with `nextZ()`, exactly as the right-click menu is.
- Verified in headless chromium against the built stylesheet: the pill renders
  five modes with Select pressed; the space bar lights Move, answers
  `panning=true` and sets `cursor: grab`, and releasing it comes back to Select;
  a space bar pressed in a field changes nothing; pressing Draw while the bar is
  held lands on Draw when it is let go; a blur releases the hold. The floors
  menu opens 224px wide at the button's top edge, ends 5px inside the table,
  truncates a 51-character floor name with an ellipsis, shows one "Active"
  badge inside the right edge, posts `/rooms/{id}/layers/{layer}/activate` for
  any floor but the live one, and closes on a choice, an outside press and
  Escape. The CSS build gained three rules and no component family:
  `.max-h-72` and the two `aria-expanded:` utilities.

## End state

- The GM opens a Spawn dialog, searches monsters or tokens, picks one, and
  clicks the map to place it, again and again until Escape. A Visible toggle
  decides whether players see it appear. *(Reworked above: a token is always an
  object and is drawn at its picture's size; the dialog asks for neither.)*
- Spawn pawns places a pawn for every connected player who joined with a
  character. *(Reworked above: it is the GM's alone and opens nothing.)*
- Anyone selects pawns they may move: click one, Shift-click to add, or drag a
  marquee over several. Dragging a selected pawn drags the whole selection as
  one. Dragging a wagon carries the pawns standing on it without selecting
  them first.
- With snapping on, the grabbed CREATURE snaps to cells or corners by footprint
  parity, the rest keep their exact offsets, a path of highlighted cells runs
  from where the grabbed pawn stood to where it will land, and a label shows
  the distance in feet under the table's diagonal rule. Every other client
  sees the same ghosts and path in the dragging player's colour. Release
  commits; Escape cancels.
- Hovering a creature shows a label: its name, hit points as the viewer may
  see them, and armour class. Selecting several shows the count, a Move to
  floor select and a Remove button for the GM. *(Reworked above: the label is
  hover-only and carries no buttons, a token gets none at all, and Delete
  presses a hidden button on the page instead. Rework 7: whether there is a
  label at all, and what a player reads in it, is the room's `pawnLabels`
  setting.)*
- The pawn window edits HP with arithmetic input, max HP, AC, a creature
  size or an object's width, height and angle, floor, visibility, and
  conditions with colour and duration. *(Reworked above: an object carries an
  angle and is never snapped, and this is a window rather than a modal -- there
  is no pawn dialog any more. Rework 6: the two hit-point numbers are one row,
  both take sums, the name is a Rename dialog, and nothing is saved by pressing
  anything.)*
- Every pawn lives on one layer. The canvas shows the viewed layer's pawns;
  spawns land on the layer the spawner is viewing; the GM moves pawns between
  layers from the pawn window's floor select or the selection overlay's Move
  to floor control. When the GM switches the active layer, players see the
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
10. **Editing a pawn is two surfaces and both post over HTTP.** The live panel
    in a window carries one editable control, HP, because that is the value
    that changes every round and a dialog in front of "the goblin takes 7" is
    the wrong shape; it posts to a route of its own and is answered with the
    panel, which is the mutation-returns-what-it-changed case the fragment
    rules name. Everything else -- name, size or footprint, max HP, AC,
    conditions, visibility, layer -- is a form in the content modal, which
    cannot be clobbered by a refetch because nothing refetches it. Both
    handlers build `pawn.update`, `pawn.setConditions`, `pawn.setVisible` and
    `pawn.setLayer` as needed and dispatch them. The HP field accepts `12`,
    `-7` or `+3`, evaluated server-side: an absolute value replaces, a signed
    value adds to the current. Errors from the core come back as 422 form
    errors.
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
| `GET /fragment/room/pawn?room={id}&pawn={id}` | `Fragment` | `RoomPawnFragment` | Any member; the projection decides what they see. Reads the live pawn via `hub.Pawn(ctx, roomID, pawnID, role)`, PROJECTED FOR THE ROLE; 404 empty if absent or not shown to the requester. Renders the panel that goes in a window, which is the pawn's whole surface: a header carrying the name and three icon buttons -- Stat block, Rename and Remove, each with a tooltip and an aria-label, and no Rename on a player's character; then readings for a viewer, and for somebody who may edit it the hit-point row plus an autosaving editor carrying creature size or pixel size and angle, AC, conditions, and for the GM visibility and a floor select. Every id in it carries the pawn's own, because two are open at once. See the last section of this document for the trigger it carries and why. |
| `GET /fragment/room/pawn/rename?room={id}&pawn={id}` | `Fragment` | `RoomPawnRenameFragment` | The rename dialog for the content modal, prefilled with the name the pawn has now. Same permission as the panel: the projection decides whether the asker may see the pawn, `mayEditPawn` whether they may change it. |
| `POST /rooms/{id}/pawns/{pawn}` | `RequireSession` | `UpdatePawn` | The panel's editor, autosaving. Parses it, dispatches `pawn.update`, then `pawn.setConditions`, then `pawn.setVisible` when the GM toggled it, then `pawn.setLayer` when the GM changed the floor, and answers with the panel's ERROR SLOT -- empty on success, the message under a 422. It carries neither the name nor the hit points. NOT `htmx.CloseModal`: there is no modal, and sending it would dismiss whatever else was open. |
| `POST /rooms/{id}/pawns/{pawn}/hp` | `RequireSession` | `UpdatePawnHP` | The hit-point row, both boxes. Evaluates the arithmetic in each against the live value, dispatches one `pawn.update` carrying whichever of the two was filled in, and answers with the error slot. An empty box is untouched rather than zero. NOT `htmx.CloseModal`: there is no modal open, and sending it would dismiss whatever else was. |
| `POST /rooms/{id}/pawns/{pawn}/name` | `RequireSession` | `RenamePawn` | The rename dialog's save. Dispatches `pawn.update` with the name alone, then `htmx.CloseModal` and 204. A route of its own because the editor's POST replaces the pawn's conditions with the rows its form carried, so one field posted there would take every chip off the goblin on the way past. |
| `POST /rooms/{id}/pawns/layer` | `RequireSession` | `MovePawnsToLayer` | GM only. Reads repeated `ids` and `layer`, at most `SelectionMax`, dispatches `pawn.setLayer`. Empty 200. Used by the pawn window with one id and by the overlay with the selection. |
| `DELETE /rooms/{id}/pawns` | `RequireSession` | `RemovePawns` | GM only. Reads repeated `ids` form values, at most `SelectionMax`, dispatches one `pawn.remove`. Empty 200. Used by the pawn window with one id and by the overlay with the selection, which the Delete key presses. |
| `GET /fragment/room/stat-block?room={id}&pawn={id}` | `Fragment` | `RoomStatBlockFragment` | GM, or any member when the pawn is visible. Resolves the pawn's `monsterId`, reads with the room owner as owner, renders the same stat block component the manual uses. |

The Spawn dialog opens in the content modal at size `lg`. The pawn panel and
the stat block are **windows**; see "Windows, and how live data reaches them"
at the end of this document, which supersedes decision 10's "dialog" and the
last two rows of the table above. Buttons are solid `btn`; Close first.

### Templates

- `pages/room-spawn.templ`, `.go`: `RoomSpawnFragment(data RoomSpawnData)` and
  the two card partials. The monster card shows image, name, CR and size; the
  token card shows the image and name. The Creature or Object choice and the
  footprint fields sit above the token results.
- `pages/room-pawn.templ`, `.go`: `RoomPawnFragment(data RoomPawnData)`. The
  conditions editor uses `repeater.js` rows: name (with a `<datalist>` of the
  twenty familiar conditions from the old client), colour select of the eight
  colours, duration number with -1 meaning until removed, clear-on select.
  Visibility and floor controls render only for `RoleGM`. Remove is a button
  with `hx-delete` to the pawns route, `hx-vals` with the id, and `hx-confirm`.
  *(Rework 4: the editor is `pages/room-pawn-editor.templ` and renders inside
  this same fragment rather than in a modal of its own.)*
- The room page gains the overlay element, hidden by default, with two blocks
  inside it toggled by `hidden`: the single-pawn block and the multi-selection
  block. The multi-selection block's Remove button carries `hx-delete`,
  `hx-confirm` with a heading override, and an `hx-vals` the client fills with
  the selected ids. For the GM it also carries a Move to floor `<select>` whose
  options the client fills from the store and whose change posts to the layer
  route with the same `hx-vals`. *(Rework 5: the single-pawn block is a name,
  hit points and armour class and carries no controls; beside the overlay the
  page renders one hidden GM-only Remove button, `data-pawn-remove`, which is
  what the Delete key presses.)*
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
dialogs.ts            listens for room:arm, opens the spawn modal and the pawn and stat block windows from the overlay
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
the band word when that is all the viewer has, and AC. *(Rework 7: whether
the label is drawn at all is the room's `pawnLabels` setting, and AC is withheld
from a monster on everything but `full`.)* For several pawns the
overlay shows the count, a Move to floor select and, for the GM, the Remove
button whose `hx-vals` the client sets to the selected ids.

*(Reworked above. Rework 5 took the condition dots and all three buttons off the
single-pawn label and made it hover-only and never a token's; the pawn window
is opened by a double click, or from the right button's menu, and the stat block
from a button in that window. The Edit button and the modal it opened went in
Rework 4.)*

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
- `UpdatePawnHP` arithmetic: `12` sets, `-7` subtracts, `+3` adds, `23-7-4`
  is worked out left to right, an empty box changes nothing, and garbage is a
  422. It answers with the error slot and sends no `HX-Trigger` that would close
  a modal. An object form with conditions is a 422.
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
7. Set pawn labels to the middle choice; the player's overlay says Bloody
   when the GM takes a goblin to half.
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

---

## Windows, and how live data reaches them

Added after phase 3, which built the window system and the player list in it.
Read the Windows section of `CLAUDE.md` first; this is what it means for the
surfaces above, and it **supersedes** decision 10's content modal for the pawn
panel and the stat block's row in the route table.

### Which surface is which

A window is not a better modal, and picking between them is one question: does
this stay open while the GM works, or is it a task they finish?

| Surface | Kind | Why |
| --- | --- | --- |
| Pawn panel: everything about one pawn, readings and controls both | **Window**, `pawn:<ulid>` | It stays open through a fight, several at once, tucked in a corner. This is what the old client opened on right-click and it is the interaction worth carrying forward. *(Rework 4: the edit form moved in here and the modal was deleted. A form that refetches would throw away what somebody was typing, which the panel's own trigger filter already prevented for its one field and now prevents for all of them. Rework 6: the filter narrowed to a box with the caret in it, and neither save swaps the panel at all.)* |
| Rename one pawn | Content modal | A task with an outcome: one field, prefilled, and it closes. The panel behind it is corrected by the socket like every other open copy. |
| Stat block | **Window**, `monster:<ulid>` | Keyed by the monster and not the pawn, so eight goblins share one window rather than opening eight identical ones. |
| Spawn dialog | Content modal | A task with an outcome: it arms placement and closes. |

The pawn panel's trigger lives on the selection overlay rather than in a menu,
because a pawn window is about one pawn and the overlay is where that pawn is
named -- and, since Rework 11, on a double click anywhere on that pawn or on
the first item of the menu the right button puts up. All three go
through `pawnWindow` in `js/room/pawn-window.ts`, which is the one place the id,
the URL, the heading and the opening size are spelled:

```go
RoomWindow{
    ID:     "pawn:" + pawn.ID.String(),
    Title:  pawn.Name,
    URL:    "/fragment/room/pawn?room=" + roomID + "&pawn=" + pawn.ID.String(),
    Width:  260,
    Height: 300,
}
```

### How a window stays live

**The socket says what changed; the GET fetches what it looks like.** The
fragment reads the hub's in-memory room, not the database, so the refetch is a
render of the same state the socket event came from -- there is no second source
of truth and no query behind it.

Two things make it scale to a table full of open pawn windows.

**The DOM event carries the id.** `panels.ts` maps `pawn.updated`, `pawn.spawned`
and `pawn.removed` to a `room:pawn` event with `detail: { id }`. Today its
events carry no detail at all, because nothing consumes one; this is a two-line
change when the first pawn window lands.

**The fragment filters on it in its own markup.** htmx's trigger filter is a
condition in brackets after the event name, evaluated with the event's
properties bound as variables and `this` bound to the element:

```html
<article id="pawn-01H..."
    hx-get="/fragment/room/pawn?room=...&pawn=01H..."
    hx-trigger="room:pawn[detail.id === '01H...' && !this.contains(document.activeElement)] from:window"
    hx-sync="this:queue last"
    hx-swap="outerHTML">
```

Ten pawn windows open and one goblin taking damage is one GET. Without the
filter it is ten.

`hx-sync="this:queue last"` is not optional and is the same rule the player
window follows: a burst of events is a burst of GETs whose answers can land in
either order, and the socket being ordered does not make a pair of HTTP requests
ordered. Never `replace` -- it cancels the request in flight and htmx reports
every cancellation as a console error.

**`document.activeElement` is what keeps a refetch from eating a keystroke.**
The one editable control in the panel is HP, because that is the value that
changes every round and a modal in front of "the goblin takes 7" is the wrong
shape. The filter above skips the refetch entirely while focus is anywhere
inside the panel, and the swap that follows the person's own POST brings the
panel back in step. Every other edit is the content modal's form, which cannot
be clobbered because nothing refetches it.

### What the server owes this

`hub.Pawn(ctx, roomID, pawnID, role)` beside the existing `Players`, returning
the pawn **projected for the requesting role** -- the same `projectPawn` the
socket calls.

**THIS IS THE ONE THING IN THE PHASE THAT MUST NOT BE GOT WRONG.** The whole
two-audience design says a hidden pawn never reaches a player's browser. An
unprojected fragment handler is a back door around it: a player guesses a ULID,
GETs the fragment, and reads a hidden monster's exact hit points that the socket
was careful never to send. The route table above already says "404 empty if
absent or hidden from the requester" and that clause is load-bearing. The test
for it is in the list below and should be written first.

The same applies to the stat block route, which is already scoped that way.

### The rule about which events may drive a refetch

**Only events that count, and never a hot path.** Hit points, armour class,
conditions, name and layer change at human pace -- a few per round -- and a GET
each is nothing.

`pawn.moved`, `pawn.dragging` and `stroke.extended` are the three hot paths in
the protocol. They fire up to twenty times a second, and binding a refetch to
one of them is precisely what "the socket carries JSON and the DOM refetches"
was designed to avoid. A window that wants a per-frame value does not fetch it:
the client already holds the whole projected room in `store.ts`, which is what
the debug panel reads. htmx renders the structure once and a small binding layer
updates a few marked spans in place. That is roughly thirty lines and it is not
worth writing until something needs it; the point of recording it here is that
the escape hatch exists and does not require touching the protocol.

### Two loose ends the window creates

**A removed pawn leaves a stale window.** The fragment 404s, `noSwap` covers
4xx, so htmx swaps nothing and the panel sits there showing a dead goblin's hit
points forever. `panels.ts` dispatches `window:close` with the id on
`pawn.removed`, and the window manager closes it. Build it with the first pawn
window rather than finding it at a table.

**A renamed pawn keeps its old title.** A window's title is set from the trigger
when it opens, so `pawn.updated` with a new name does not reach it. The fix is
the mirror of the above: `window:retitle` with an id and a title, and one line
in the manager that sets the heading's `textContent`.

### Responsiveness inside a window

The panel is 260px wide by default and can be dragged to 900. **Style it with
container queries, and let the fragment declare its own container** -- not the
window:

```html
<article class="@container">
    <dl class="grid grid-cols-1 @sm:grid-cols-2 gap-2">
```

Tailwind 4 has these in core (verified against the vendored 4.3.3 binary:
`@container`, named containers, `@min-[420px]:` and `cqi` units all emit). The
reason the container belongs to the fragment is that a container query with no
query container above it evaluates to *unknown*, which is false -- so a fragment
that relied on the window declaring one would render permanently in its narrow
state anywhere else, including in the content modal and on a full page. A
fragment that carries its own container is sized by its own box everywhere.

`container-type: inline-size` queries width only. Height queries need
`container-type: size`, which needs a definite height; a window body has one and
a page section usually does not.

### Tests this section adds

**Go**:

- `hub.Pawn` projects: a player asking for a hidden pawn gets nothing, and a
  player asking for a visible monster with the default setting gets a word, no
  armour class and no
  numbers. This is the security test and it is the first one to write.
- The pawn fragment renders its `hx-trigger` with the pawn's own id in the
  filter and with `hx-sync="this:queue last"`, so a refactor cannot quietly turn
  ten windows into ten GETs or reintroduce the ordering race.

**TypeScript**:

- `panels.test.ts`: `pawn.updated` dispatches `room:pawn` carrying the pawn id;
  `pawn.removed` dispatches both that and `window:close`; none of
  `pawn.moved`, `pawn.dragging` or `stroke.extended` dispatches anything at all.
  That last assertion is the hot-path rule, written down where it fails.

### Verification this section adds

12. Open the pawn panel for two goblins. The GM takes seven off the first: its
    window updates and the second window makes no request (check the network
    panel -- one GET, not two).
13. Click into the first window's HP field and type. The GM's other tab changes
    that pawn's AC; the field keeps what was typed. Blur it, change the AC
    again, and the panel catches up.
14. A player opens a panel for a visible monster with labels on the default
    setting: it says Bloody, with no numbers and no armour class. The GM hides that monster; the player's
    window is closed by `pawn.removed` rather than left showing stale stats.
15. Drag the pawn panel to 900px wide: the stat rows go from one column to two
    without a reload.
