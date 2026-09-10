# Phase 9: pings

Read `plans/phase-2-protocol-core.md` for the ping command, and
`plans/phase-8-drawing.md`, whose pill button, tool wiring and pass placement
this phase copies. This phase lets anybody at the table point at it.

**Written 2026-09-10** against phase 8's shipped code, replacing the sketch
that was carried over when the old phase 6 was split on 2026-09-09 and split
out of phase 8 on 2026-09-10.

Four things the sketch left open are decided here and four of its decisions
moved. Decided: the modifier question (there is no modifier gesture, decision
1), the name question (there is no name, decision 7), the camera question
(nothing moves, decision 11) and the easing. Moved, each marked *Changed*
below: the ring converges rather than expands, the sound is synthesised rather
than a file, the sound's control is not a pill button, and there is no ping
options pill at all.

**Both checkpoints built 2026-09-10**, and two things changed at the table
after they were. The sound shipped as a single boop -- the envelope, not the
pitches; see decision 8 -- and the mute became a volume in the account
settings, which supersedes decision 9 outright. Both are written up where they
happened rather than at the top, because what is worth keeping is the argument
that turned out to be wrong and not the fact that one did.

## Already built

**The server is finished and this phase does not touch it.** That is worth
saying plainly because it is not true of any other phase: `internal/room/ping.go`
holds `Ping{Layer, X, Y}` and `Pinged`, `Authorize` is `requirePlayerLayer` and
so lets **anybody** ping a floor they are looking at, `Apply` bounds both
coordinates through `checkCoord`, and the emission is `ToAll` with the actor
stamped into the header by the hub. `Transient() bool { return true }` keeps it
out of the snapshot, out of the sequence check in `socket.ts`, and out of the
reducer -- `store.ts` names `pinged` in the transient arm beside `pawn.dragging`.

It is tested already, and by the tests this phase would otherwise have to
write: `authorize_test.go` has all three roles pinging, `layer_test.go` refuses
a player pinging a floor they are not on and allows the GM, `validate_test.go`
walks both coordinates to `CoordLimit` and one past it, `reduce_test.go` proves
a `Pinged` changes no state, and `scenario_test.go` has a player pointing at a
door mid-session.

Spam is bounded below the feature: `internal/hub/conn.go` runs a per-connection
token bucket over every command, so a ping held down on a script is refused by
the socket rather than by anything written here.

Client, all of which this phase reuses:

- `render/ring-pass.ts` draws hollow ellipses at any radius, with a colour, an
  alpha and a thickness in **device** pixels, and it is already called three
  times a frame (condition rings, outlines, handles). A fourth batch is a
  `begin`/`add`/`draw`.
- `render/decals.ts` is the exact shape a transient per-viewer animation takes
  here: a pool keyed by floor, `build` into a target interface declared in the
  module so the module holds no WebGL, `settling(now)` answering the frame
  loop, and no timer anywhere.
- `render/frame.ts` renders one frame per `invalidate()` and asks for another
  only while `drawFrame` returns true. A finite animation keeps itself alive
  and then lets the room go quiet.
- `actorColor(id)` in `pawns.ts`: eight colours from a hash of a player id,
  already the colour of that person's drag ghosts, their ruler, and the pen
  they start on.
- `tools.ts` reads what a pill button does off `data-room-tool-pans`,
  `-measures`, `-fogs`, `-draws` and its letter off `data-room-tool-key`, and
  answers `panning()`, `measuring()`, `fogging()`, `drawing()`.
- `main.ts`'s `fanOut` hands every event to each entry in turn, so a new family
  of events is one entry rather than a branch inside somebody else's.
- The Tabletop menu already carries `Clear blood` as a `data-room-action`, and
  `public/js/room.js` turns one of those into a window event the room bundle
  hears. `internal/events` and `public/js/events.js` are the two halves of that
  contract and a test pins them together.

Not built: any ping rendering, a Ping button, `pinging()`, and the sound.

## End state

- Anybody may choose **Ping** in the pill -- sixth button, key `p`, for every
  role -- and a press on the table points at that spot for everybody looking at
  that floor.
- A ping is **three rings converging on the point** over about a second, in the
  pinger's own colour, drawn over the creatures and under a player's fog cover.
- It appears **in the same place at the same time on every screen**, including
  the pinger's, because it is drawn from the wire and never locally.
- A ping **makes a short blip** -- two rising notes -- except your own and
  except one on a floor you are not looking at.
- **Settings > Sounds > Ping volume** runs it from full down to silent, and the
  bottom of that slider is the only mute there is.
- Nothing about a ping is stored, restored, reduced, or in a snapshot, and a
  room nobody is pinging costs one branch a frame.

## Decisions

1. **Ping is a pill button, it is everybody's, and there is no modifier
   gesture.** *Kept, and the sketch's open question is closed against the
   modifier.* Sixth button after Draw, key `p`, no `GM` flag; `tools.ts` grows
   `pinging()` beside `drawing()` and `RoomTool` grows `Pings` beside `Draws`.

   **The sketch asked whether Alt-click in any tool would be better than a mode
   of its own, and the answer is that every modifier is already spoken for.**
   Shift is the marquee, additive selection, the rotation step and the resize
   aspect; Alt is `withRiders` on a pawn drag and corner snapping in the fog;
   Ctrl and Cmd are undo and the browser's own; the space bar is the pan
   borrow. A chord that means "ping" in Select and "leave the riders behind" in
   a drag is worse than a mode, because it is a gesture whose meaning depends
   on what the hand happens to be over.

   **AND A FORGOTTEN PING MODE IS THE SAFEST FORGOTTEN MODE IN THE APP**, which
   is what makes a mode acceptable here where it would not be for the eraser. A
   stray press in Select moves a goblin, in Fog cuts a hole, in Draw lays ink;
   a stray press in Ping puts a ring on the table for one second and changes
   nothing. There is nothing to undo because there is nothing to undo.

   The pill also gets a tooltip and a printed key, which is where a gesture is
   taught. A held chord is taught nowhere.

2. **The gesture is one press and it lives in `pawns.ts` rather than in a
   module of its own.** `fog.ts` and `draw.ts` are modules because each owns
   geometry, a read of the store, a keyboard and a multi-event gesture. A ping
   owns none of those:

   ```ts
   if (deps.pinging()) {
       deps.send({ type: "ping", layer: deps.viewed(), x: Math.round(map.x), y: Math.round(map.y) });
       return true;
   }
   ```

   That is the whole tool. `drag` and `release` do nothing in this mode, there
   is nothing for `secondary` or `abandon` to put away, and `active()` stays
   false. A module for it would be four lines of behaviour behind forty lines
   of dependency injection and a wiring block in `main.ts`.

   **IT FIRES ON THE PRESS AND IT CLAIMS THE PRESS.** On the press because the
   table acts on presses and a ping is a tap. Claiming it because the
   alternative -- returning false and letting the camera have the gesture too --
   means every attempt to shove the map sideways in this mode throws a ping at
   wherever the drag started. The middle button and the space bar still pan,
   which is how you move around without leaving the tool.

   The coordinates are **rounded**, because the protocol carries integers and
   `checkCoord` is an integer bound.

3. **A ping converges rather than expands.** *Changed.* The sketch had one ring
   growing from a quarter of a cell to two cells while it faded. This is three
   rings shrinking the other way -- from two cells onto a quarter of one --
   staggered so they arrive one after another.

   **A GROWING RING IS A RIPPLE AND A SHRINKING ONE IS A TARGET.** The eye
   follows the moving edge, so a ring that expands leads the eye away from the
   thing being pointed at and leaves it nowhere in particular; a ring that
   closes puts the eye on the square. What a ping means is "look **here**", and
   the animation should end where the sentence does.

   The expanding version is better at one thing -- it sweeps more area, so it
   is easier to catch in peripheral vision -- and the stagger is what buys that
   back: three arrivals over a second is three chances to notice, covering the
   same ground as one sweep.

   Starting constants, and the sketch was right that these are a thing to look
   at rather than to settle in a document:

   ```ts
   const RINGS = 3;          // per ping
   const RING_MS = 700;      // one ring's whole life
   const STAGGER = 180;      // between one ring and the next
   const RADIUS_MAX = 2;     // cells, where a ring starts
   const RADIUS_MIN = 0.25;  // cells, where it ends
   const WIDTH = 2;          // device pixels: RING_WIDTH, the condition ring's
   const FADE = 0.25;        // the last quarter of a ring's life
   ```

   A whole ping is therefore `RING_MS + (RINGS - 1) * STAGGER`, about 1.06
   seconds. **A SECOND RATHER THAN THE SKETCH'S TWO**: long enough to be caught
   out of the corner of an eye, short enough that it can never become furniture
   on the map.

   The radius eases out -- `1 - (1 - t)^3` -- so a ring moves fast and then
   settles onto the point, which reads as landing rather than as sliding. Alpha
   is flat until the last quarter and then goes to nothing, so the ring is at
   full strength while it is doing the pointing.

4. **The rings live in `render/pings.ts`, and that module is `decals.ts`'s
   shape.** A pool keyed by floor; `add(layer, x, y, by, now)`; `build(layerID,
   now, into)` where `into` is a `RingTarget` interface declared in the module
   rather than an imported pass; `settling(now)` for the frame loop; `clear()`.

   **THE INTERFACE IS WHY IT HOLDS NO WebGL AND ITS TESTS NEED NO CANVAS**,
   which is the reason `decals.ts` does it and is worth more here than there:
   everything interesting about a ping is arithmetic over time, and arithmetic
   over time is exactly what a headless test can check.

   **NO TIMER.** `settling` answers the render function, the render function
   keeps the loop alive, and the loop stops on its own when the last ring is
   gone. A `setTimeout` would keep a tab awake that has nothing to draw, which
   is the promise `frame.ts` exists to keep.

   `CAP = 16` live pings per floor, oldest dropped. Six people at a table
   cannot make more than that in a second by hand, and a script is refused by
   the socket's token bucket before it gets here -- so the cap is a bound on a
   bug rather than on a person.

5. **A ping is drawn over the creatures and under a player's cover.** In
   `drawPawns`, after the handles' batch and **before** `drawFog("player")`.

   **OVER THE CREATURES BECAUSE THE POINT OF A PING IS OFTEN A CREATURE.** The
   drawing goes under them -- a circle round three goblins has the goblins
   standing in it -- but a ring converging on a square somebody is standing on
   has to be visible over the token standing there, or it points at nothing.

   **UNDER THE COVER BECAUSE A PING IS SOMEBODY ELSE'S AND CAN POINT INTO
   FOG.** The marks and labels above it in the draw order go over the player's
   cover, and the comment there says why: a measurement is the viewer's own
   mark on their own screen, so reading it over the fog is how somebody
   measures the distance to a door they have not opened. A ping is the opposite
   -- it arrives from another person and lands wherever they pressed -- so a GM
   who pings into an unrevealed room must be pointing at nothing as far as the
   players are concerned. Same rule as a stroke, for the same reason.

   The batch is skipped entirely when no ping is live, so a table nobody is
   pointing at costs one branch a frame.

6. **The ping goes to everybody including the pinger, and the pinger's own is
   drawn from the wire.** *Kept.* This is already the server's decision and the
   reasoning is in `ping.go`: drawing your own locally would put your marker on
   the map a round trip before everybody else's, and the one thing a ping has
   to be is in the same place at the same time on every screen.

   It costs the pinger about 30 ms of nothing on a local socket, and it buys a
   feature with no second code path in it.

7. **The ring carries no name; the colour is the name.** *The sketch's open
   question, decided against the name.*

   `render/glyphs.ts` rasterises `0123456789 ft.` and nothing else, so a name
   is either an atlas of letters or a DOM element positioned in map space every
   frame. Both are real work, and both put a word on the map for one second.

   **THE COLOUR IS ALREADY THIS PERSON'S EVERYWHERE ELSE ON THE TABLE**: their
   drag ghosts, their ruler, and the colour their pen opens on. A table of five
   learns eight colours in one session without being taught them, which is what
   `actorColor` was for. And the person who pinged is almost always talking at
   the same moment -- a ping is punctuation on a sentence somebody is saying
   out loud, not a message on its own.

8. **The sound is synthesised rather than played from a file.** *Changed.* The
   sketch had one reused `Audio` element on `/static/ping.mp3`.

   **A FILE IS A BINARY ASSET, A ROUTE, A CACHE, AND A 404 NOBODY NOTICES.**
   What it buys is a sound somebody chose; what it costs is a thing in the
   repository that cannot be read, diffed or reviewed, and a failure mode --
   the fetch that quietly does not land -- whose only symptom is silence, which
   is also what a working mute looks like.

   Instead: one lazily created `AudioContext`, one sine oscillator through one
   gain node, a two-note rise. Pitch, length and volume become constants that
   can be changed by editing a line.

   **AND THE FIRST BUILD OF IT WAS A SINGLE BOOP, WHICH IS WORTH RECORDING
   BECAUSE THE FREQUENCIES WERE NEVER THE PROBLEM.** The pitch was stepped to
   the second note 45 ms into a 90 ms note whose gain was already decaying
   exponentially from its peak -- so the second note arrived at 2.5 percent of
   peak, 32 dB down. Scheduled, and inaudible. The fix is the envelope: hold the
   level flat across both notes, dip to nothing between them, and decay only on
   the tail. `voice()` is exported as a pure function over an `AudioRamp`
   interface so the shape can be tested, and the two assertions that matter --
   the level rises to peak twice, and the second pitch lands between a dip and a
   rise rather than under a fade -- both fail on the original.

   ```ts
   const LOW = 1046.5;   // C6
   const HIGH = 1568.0;  // G6
   const STEP_MS = 45;   // where it goes from one to the other
   const BLIP_MS = 90;
   const GAIN = 0.25;
   ```

   **THE CONTEXT IS MADE ON THE FIRST PING AND NOT AT MOUNT**, because a
   context created before any user gesture starts suspended, and because a room
   whose table is silent all evening should not have allocated an audio device.
   Every call is wrapped: a browser that refuses, a context that will not
   resume, or a viewer who has never clicked all end in silence rather than in
   an exception on the socket's fan-out.

9. **How loud is an account setting, and the bottom of the slider is the
   mute.** *Superseded 2026-09-10, after this was built the other way and run
   at a table.*

   **WHAT THIS DECISION SAID FIRST, AND WHY IT WAS WRONG.** It put a
   `Mute pings` row in the Tabletop menu backed by `localStorage`, on the
   reasoning that a sound is about the room somebody is sitting in -- headphones,
   a voice call, somebody asleep upstairs -- rather than about the person, and
   that the moment you want to mute is the moment somebody **else** is pinging,
   which rules out a pill that is only on screen while its own tool is chosen.

   That reasoning about *placement* still holds and the room's menu was a
   reasonable home for a switch. What it got wrong is that the setting is not a
   switch. A ping that is too loud is not a ping you want silenced; it is a ping
   you want quieter, and a binary control cannot say so. Once the answer is a
   number, the argument for keeping it out of the account dies with it: a volume
   is exactly the kind of thing somebody sets once and expects to find set on
   their other machine, which is `ShowBlood`'s case and not the exception to it.

   So: a **Sounds** section in the settings dialog, one range from silent to
   full, and `users.ping_volume` beside `follow_turn` and `show_blood`. The menu
   row, its id and `RoomMenuItem.AltLabel` were removed with it -- **a slider
   whose bottom stop is silence already answers "at all" as well as "how loud",
   and two controls over one setting are two things that can disagree about
   it.** There is now no ping control in the room, and a test pins that.

   **THE COST IS REAL AND IT IS THE ONE THING TO WATCH.** Muting mid-session is
   now Help > Settings, a slider and a save, rather than two clicks in the menu
   over the table. If that turns out to bite, the answer is a room-local
   *session* mute layered over the account's standing level -- not a second
   control that writes the same value.

   The cross-bundle event this decision used to need is gone with the row. It
   costs a migration, a seventh field on `prefs.Preferences`, a `pingVolume` on
   `htmx.Settings`, and `data-ping-volume` on the tabletop, none of which is new
   machinery: it is `ShowBlood`'s path with a number on it.

   **THE CURVE IS SQUARED AND THE MUTE IS EXACT.** Halfway down a linear gain
   slider is six decibels, which reads as "slightly less" and puts every setting
   anybody would choose in the bottom third of the travel; squared is twelve,
   which is about half as loud to an ear. Zero is zero rather than very nearly
   zero, because it is the mute and a curve that merely approached silence would
   be audible through headphones in a quiet room.

10. **Your own ping is silent, and so is one on a floor you are not looking
    at.** Both are drawn-or-not decisions made in `main.ts`, which is the one
    place that holds the viewer's id and the viewed floor.

    Your own is silent because you already know: the ring **is** the
    confirmation that the round trip landed, and a blip on every one of your
    own presses turns a tool into a noise.

    Another floor is silent because a sound with no visible cause is the worst
    feedback there is -- the viewer hears something, looks at the map, and
    nothing happened. A player can only ever ping the floor they are on, so
    this only arises for a GM working on another floor while the party points
    at the ground floor; telling them about it is in Out of scope, and if it is
    ever built it wants to be a message rather than a mystery blip.

11. **Nothing moves anybody's camera.** *The sketch's open question, decided
    against.* The renderer has the machinery -- `follow.ts` already travels the
    camera onto whoever is acting -- so this is a decision rather than a
    limitation.

    A ping that yanks the viewport is a ping that interrupts, and every person
    at the table can send one. It would land mid-drag, mid-measurement, and
    mid-read of a stat block. The feature is "look here", and a person who
    wants to look can; a person who does not want to should not be moved.

    The honest gap is a ping outside your viewport, which is a ring nobody
    sees. The sound is what covers it -- somebody pointed, and the person who
    pointed is talking -- and the fix if that turns out not to be enough is an
    edge indicator rather than a camera move.

## Server

Nothing. Not a field, not a command, not a migration, not a snapshot step, not
a test. `ping.go` is already exactly this feature and the tests listed under
Already built already cover it.

The one Go change in the phase is `internal/events`, which is not the room: one
constant, and it is there because the two browser bundles cannot import each
other.

## Client

### Modules

- **`js/room/render/pings.ts`** (new). The pool: `add`, `build`, `settling`,
  and a `RingTarget` interface. No WebGL, no timer, no DOM. *As built it also
  exports `ringAt`, a pure function of one ring's age, because that is the part
  worth testing; `clear` was dropped, because nothing needs it.*
- **`js/room/ping-sound.ts`** (new). The blip and the level: `play()` and
  `volume(percent)`, plus the exported `voice()` and `gainFor()` the tests
  drive. It owns its `AudioContext` and NOTHING ELSE -- no storage, no DOM, no
  control of its own -- and every entry point is wrapped.
- **`js/room/tools.ts`**. `pinging()`, read off `data-room-tool-pings`, false
  on a page with no such button.
- **`js/room/pawns.ts`**. A `pinging: () => boolean` dep and the four lines in
  `press` from decision 2.
- **`js/room/render/renderer.ts`**. Creates the pool; `pinged(layer, x, y, by)`
  on the `Renderer` interface beside `bloodCleared`; the fourth ring batch;
  `pings.settling(now)` in `drawPawns`'s return.
- **`js/room/main.ts`**. One `fanOut` entry, and it is where both silences in
  decision 10 are decided:

  ```ts
  (event) => {
      if (event.type !== "pinged") return;
      renderer?.pinged(event.layer, event.x, event.y, event.by ?? "");
      if (event.by !== user && event.layer === viewed()) sound.play();
  },
  ```

  *As built this is a `pinged` callback handed to `start()`, because the two
  silences need the viewer's id and the viewed floor and `start()` holds
  neither.* It also reads `data-ping-volume` at mount and applies `pingVolume`
  from every `settings:change`.

### Server, for the volume

Decision 9's supersession is the only server work in the phase and it is
`ShowBlood`'s path with a number on it: a `ping_volume` column, a seventh field
on `prefs.Preferences` with a forgiving `ClampPingVolume` for the read path and
a strict `ParsePingVolume` for the write path, `pingVolume` on `htmx.Settings`,
`data-ping-volume` on the tabletop, and a **Sounds** fieldset holding one range.

The fixtures that had to learn the column are worth naming, because three of
them fail in ways that do not mention it: `middleware/session_db_test.go`'s
fake driver (whose own comment predicts exactly this), `controllers/
account_test.go`'s `settingsForm` and its two column-list assertions, and
`templ/pages`'s `testRoomPage` -- which needs the field set at all, because
**this one's zero value is silence.**

### Templates

- `templ/pages/room.go`: `RoomToolPing = "ping"`, `RoomTool.Pings`, the sixth
  entry in `RoomTools()` (`{Name: RoomToolPing, Label: "Ping", Pings: true,
  Key: "p"}`, no `GM`), and `PingVolume` with its `PingVolumeAttr()`.
- `templ/pages/account.templ`: the **Sounds** fieldset and its range.
- `templ/pages/room.templ`: `data-room-tool-pings?={ t.Pings }` beside the
  other four, and a `case RoomToolPing` in `roomToolIcon`.
- `templ/pages/icons.templ`: `pingIcon` -- concentric arcs closing on a dot.

No comment goes in either `.templ`, and the selector diff runs after both.

## Checkpoints

Two, each one a thing that can be run and played with. `make run` after each;
`make check` and the selector diff are part of both and are not repeated.

### Checkpoint 1 -- the ring

Everything visual: the pill button, `pinging()`, the press, `render/pings.ts`,
the ring batch, the frame loop, the fan-out entry. No sound.

**Verify**: two browsers on one room, a GM and a player. Choose Ping and press:
three rings converge on the spot on **both** screens, in the presser's own
colour -- the same colour their drag ghost is drawn in for the other person.
`p` chooses the tool and the tooltip prints the key. Press in Select mode:
nothing pings. Hold the space bar in Ping mode and drag: the map pans and no
ping is sent. Press on a goblin: the rings are drawn **over** it. Ping five
times quickly: the rings overlap and each expires on its own. Leave the room
alone for ten seconds and watch the debug panel: the frame loop has stopped.
GM switches to floor 2 and the player pings floor 1: the GM sees nothing. GM
pings into an unrevealed room: the player sees nothing there. Ping at the far
corner of a large map, then pan: the ring is on the map and not on the screen.

### Checkpoint 2 -- the sound

`ping-sound.ts`, the event name in both halves, the `run` case, the menu item
and its label.

**Verify**: the other browser's ping makes a short rising blip; your own makes
none; one on a floor you are not looking at makes none. `Tabletop > Mute
pings` silences it and the item now reads `Unmute pings`; reload and it is
still muted and still reads `Unmute pings`. A private window opens unmuted and
the console is clean. Ping ten times as fast as the mouse allows: the blips
stay separate rather than running into a drone. Mute, ping: the ring still
draws.

## Tests

**TypeScript**

- `pings.test.ts`: a ping's rings shrink monotonically and end inside
  `RADIUS_MIN`; `settling` is true while any ring lives and false one
  millisecond after the last; `build` emits only the named floor's rings; two
  ids get two colours and the same id twice gets one; the pool caps at `CAP`
  and drops the oldest; a ping added and then `clear`ed emits nothing.
- `pawns.test.ts` gains: a press in ping mode sends exactly one `ping`, at the
  **rounded** map point, on the **viewed** layer, and claims the press; the
  same press moves no pawn, selects nothing and clears nothing; a drag and a
  release after it send nothing more; with ping mode off no `ping` is sent.
- `tools.test.ts` gains nothing, *as built*: node has no DOM and that file says
  so, so `pinging()` is verified by a headless probe against the real dumped
  room page -- including that the space bar borrows the pointer without
  unchoosing Ping.
- `ping-sound.test.ts`: the level rises to peak once per note; the second pitch
  lands between a dip and a rise rather than under a fade; the only fade is the
  tail; nothing ramps exponentially to or from nothing; turning it down changes
  the level and not the shape; both ends of the slider are exact; the middle is
  quieter than half; an out-of-range setting is clamped and an unreadable one is
  FULL rather than silent.

**Go**

- Nothing in `internal/room`. Say so in the review: `Ping` is covered by
  `authorize_test.go`, `layer_test.go`, `validate_test.go`, `reduce_test.go`
  and `scenario_test.go`, all of which pass today.
- `internal/events` needs nothing, *as built*: decision 9's supersession took
  the cross-bundle event with it.
- Template tests: exactly one tool carries `data-room-tool-pings`; the Ping tool
  renders for a player as well as a GM and answers to `p`; every tool draws a
  different icon -- which is not a nicety, because `roomToolIcon` has a DEFAULT
  case and a tool whose case is forgotten renders as a second Select button with
  nothing in the build to say so. Plus, for the volume: the room offers no ping
  control at all, the page carries `data-ping-volume` and clamps it, the Sounds
  slider opens on what is stored, and **every position the slider offers is one
  `ParsePingVolume` accepts** -- a form that cannot be saved from one of its own
  stops is the failure that pins the step and the modulus together.

## Out of scope

A camera pull, and an edge indicator for a ping off screen (decision 11).
Telling a GM on another floor that somebody pointed (decision 10). Naming the
pinger on the table, and the letters in the glyph atlas that would take
(decision 7). A room-local session mute layered over the account's standing
level, which is the answer if reaching the settings dialog mid-session turns
out to bite (decision 9). A ping pinned to a pawn, so that it follows the
goblin rather than the square. A ping that persists until dismissed -- that is
drawing, and
drawing is `plans/phase-8-drawing.md`. A per-player mute enforced by the
server, which would be the room deciding what one person hears. A ping in the
initiative flow, and any sound for anything other than a ping.
