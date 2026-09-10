# Phase 9: pings

Read `plans/phase-2-protocol-core.md` for the ping command and
`plans/phase-8-drawing.md` for the pill button and the options pill this
copies. This phase lets anybody at the table point at it.

THIS IS A SKETCH, NOT A PLAN. It is the ping half of what was carried over
when the old phase 6 was split on 2026-09-09, kept here so nothing decided in
it is lost, and split out of phase 8 on 2026-09-10 because it shares nothing
with drawing but a pill button -- a two-second fading ring should not be in
the way of verifying the shape tools. It is to be written properly against
phase 8's code before any of it is built.

## Already built

- `internal/room/ping.go`: `Ping{Layer, X, Y}` is **anybody's** on a floor
  they are looking at, and `Pinged` goes `ToAll` **including the sender**,
  transient, with the actor in the header. Nothing on the server needs
  changing.
- `actorColor(id)` in `pawns.ts`: eight colours from a hash of a player id.
- `render/ring-pass.ts` draws hollow ellipses at any radius with a colour and
  an alpha.
- The pill's tools carry a behaviour flag and a key, read out of the markup by
  `tools.ts`; phase 8 adds `Draws` beside `Pans`, `Measures` and `Fogs`.
- `frame.ts` keeps the loop alive through `active()` and renders nothing when
  a room is idle.

Not built: any ping rendering, a Ping button, and the sound.

## What the sketch decided

1. **Ping is a pill button and it is everybody's.** The old plan put it on a
   strip that no longer exists. It is the sixth button after phase 8's Draw,
   with the key `p`, and no `GM` flag. `tools.ts` grows `pinging()` beside
   `drawing()`.
2. **A ping is a two-second ring through the ring pass** in the pinger's
   `actorColor`, growing from a quarter of a cell to two cells in radius while
   it fades, drawn only when it names the floor being viewed. The frame loop
   stays alive through `active()` while any ring lives, and there is no timer
   -- a finite animation that ends is `render/decals.ts`'s rule.
3. **The sound is a viewer's preference in `localStorage`**, wrapped in try
   and catch, default on, played from one reused `Audio` element on a short
   clip at `/static/ping.mp3` at a quarter volume, and muted from a button on
   the Ping options pill.
4. **The ping goes to everybody including the pinger**, which is already the
   server's decision and its reasoning is in `ping.go`: drawing your own
   locally would put your marker on the map a round trip before everybody
   else's, and the one thing a ping has to be is in the same place at the same
   time on every screen.

## Still to decide when this is planned properly

- Whether a ping is also a modifier-click in any tool -- Alt-click, say --
  rather than only a mode of its own, and whether that is worth a sixth pill
  button costing what it costs.
- Whether a ping names the person who sent it. The glyph atlas holds
  `0123456789 ft.` and no letters, so a name is a DOM overlay or an atlas
  change; the colour may be enough at a table of five.
- Whether a ping should pull a viewer's camera to it, and if so whether that
  is the GM's alone.
- The exact easing of the ring, which is a thing to look at rather than to
  decide in a document.

## Out of scope

A per-player ping mute enforced by the server, a ping that persists, a ping
that pins to a pawn, and drawing, which is `plans/phase-8-drawing.md`.
