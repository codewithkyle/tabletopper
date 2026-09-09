# Phase 8: strokes and pings

Read `plans/vtt-overview.md` (Drawing and erasing under Table features),
`plans/phase-2-protocol-core.md` for the stroke and ping commands, and
`plans/phase-7-fog.md`, whose options pill, tool wiring and pass placement
this reuses. This phase lets anyone the table allows draw on it and lets
anyone point at it.

THIS IS A SKETCH, NOT A PLAN. It is the strokes and pings half of the plan that
was split on 2026-09-09 into phases 6, 7 and 8, carried here so that nothing
decided in it is lost, and corrected where phase 5 and the split changed the
ground under it. It is to be rewritten against phase 7's code -- the fog tool
is the first tool with an options pill and the first pass placed by role, and
both of those are patterns this phase copies -- before any of it is built.

## Already built

- `internal/room/stroke.go`: `Stroke{ID, By, LayerID, Color, Width, Points,
  Done}`. The client mints the id and the server checks only that it parses
  and is unused. `stroke.begin` is the GM's, or a player's when
  `Table.PlayersCanDraw`, on the active floor for a player; `stroke.extend`
  and `stroke.end` are the author's, the GM included; `stroke.erase` takes a
  list and is the GM's or an author's own; `stroke.clear` names a floor.
  `StrokeChunkMax` 512 numbers, `StrokePointsMax` 20,000, `StrokesMax`
  5,000, `StrokeWidthMax` 64. `stroke.extended` is the third hot path and
  the reducer appends it.
- `internal/room/ping.go`: `ping` on a floor a player is looking at, `pinged`
  ToAll including the sender, transient, with the actor in the header.
- `stroke.cleared` is already consumed once: `main.ts` wipes a floor's blood
  on it, and `table.clear` sends one per floor.
- `Table.PlayersCanDraw` is on the Grid & settings form.
- The pill's Draw button, with no behaviour flag and no key. There is no Ping
  button.
- `actorColor` in `pawns.ts`: eight colours indexed by a hash of the player
  id, used for other people's drag ghosts and rulers.
- `ring-pass.ts` draws ellipses at any radius; `path-pass.ts` draws lines with
  a halo; the glyph atlas holds fourteen characters and no letters.

## What the sketch decided, corrected

1. **Strokes render as instanced segments with round caps in the shader.**
   Each segment carries its two endpoints, width and colour; the fragment
   shader computes the distance to the segment and produces a round-capped,
   anti-aliased line. One draw for every finished stroke on the viewed floor
   from a buffer that grows when a stroke ends and is rebuilt on `erased`,
   `cleared` and a change of floor; one small draw for strokes in progress,
   rebuilt per frame while any exists. A few hundred thousand segments is an
   ordinary frame.
2. **Erasing removes whole strokes.** The eraser tests the pointer against
   segments on the CPU inside a bounding-box prefilter and sends one
   `stroke.erase` per pointer-up with every id it touched. A pixel eraser
   would make the texture the truth and lose undo.
3. **Points are decimated on input.** With `getCoalescedEvents`, a point is
   kept only when it is at least one map pixel from the last kept point at
   the current zoom; chunks go out every 100 milliseconds or 64 points,
   whichever comes first.
4. **Draw is the pill's fifth button, key `d`, for everybody**, and Erase is
   a mode on its options pill rather than a button of its own -- the pill
   already has five buttons and the Fog tool's second pill is exactly the
   place a Draw or Erase switch, eight colour swatches and a width control
   belong. The swatches are `actorColor`'s palette, moved to a module both
   can import, so a player's strokes default to the colour their ghosts are
   drawn in. A player whose room has turned drawing off is refused by the
   core with an alert; hiding the button from them is a later courtesy that
   would have to follow `table.updated` live.
5. **Ping is a sixth pill button, key `p`, for everybody.** The old plan put
   it on a strip that no longer exists. A ping is a two-second ring through
   the ring pass in the pinger's `actorColor`, from a quarter cell to two
   cells in radius while fading, drawn only when it names the viewed floor;
   the frame loop stays alive through `active()` while any ring lives.
6. **Ping sound is a viewer's preference in `localStorage`**, wrapped in try
   and catch, default on, played from one reused `Audio` element on a short
   clip at `/static/ping.mp3` at a quarter volume, and muted from a button on
   the Ping options pill.
7. **Undo is Ctrl+Z in Draw or Erase mode**, sending `stroke.erase` for the
   viewer's newest finished stroke on the viewed floor, which is the same
   gesture phase 7 gives fog.
8. **Clearing a floor's drawing is the GM's, from the Draw options pill**,
   through a hidden button carrying `hx-confirm` and `hx-delete` to
   `DELETE /rooms/{id}/layers/{layer}/strokes`, whose `hx-vals` the client
   rewrites with the viewed floor -- the trick `data-pawn-remove` already
   uses, because the viewed floor is not in any markup and `htmx.ajax`
   cannot carry a confirm.
9. **Strokes are under the fog for players.** Phase 7 draws the cover after
   everything but the ruler, and the stroke pass goes before it, so a note
   the GM draws in an unrevealed room stays unrevealed.
10. **The own stroke is drawn from local points until `stroke.ended`.** The
    echo of `began` and `extended` for the viewer's own stroke is ignored
    until the end arrives, at which point the store's copy replaces the
    local one and the finished buffer takes it.

## Still to decide when this is planned properly

- Whether a player's Draw button should disappear live when the GM turns
  drawing off, and what the pill does with a mode that has just been taken
  away.
- Whether the ULID minted in the client is a small hand-written function
  (48-bit time plus 80 random bits, Crockford base32) or a dependency.
- The erase hit radius, in device pixels, and whether Erase shows a cursor.
- Whether stroke chunks ever need the binary frame the overview left a door
  open for; nothing measured so far says so.

## Out of scope

Pixel erasing, layers of drawing within a floor, text on the table, spell
templates, and a per-player ping mute enforced by the server.
