# Room client refactor

The core VTT features have landed. This document is the plan for reshaping
`server/js/room/` from a feature-complete first pass into a foundation that
new features plug into without editing the renderer, the pointer state
machine, or each other.

It is a working document. Tick items off as they land. Delete it when the
last phase is done. Nothing that ships may reference it.

## Goals

1. **Adding a visual is adding a stage.** A new thing on the table (lighting,
   vision, monster pawns, weather, a new overlay) is one file under
   `render/stages/` plus one entry in the draw order. It does not edit
   `renderer.ts`, does not widen the `Renderer` interface, and does not thread
   a new dependency through `main.ts`. A visual that reacts to a socket event
   handles the event in its stage; it does not add a method to `Renderer` and
   an effect to `main.ts`.
2. **Adding an interaction is adding a tool.** A new pointer mode is one file
   implementing `Tool`, registered with the tool switch. It does not edit the
   pawn gesture code.
3. **Rendering is a leaf.** `render/` imports `model/`, `gl/` and
   `protocol.ts` and nothing else from the room. The interaction layer imports
   `model/` and the renderer's public interface. Nothing imports upward.
4. **GL boilerplate is written once.** Instanced quads, growable buffers,
   programs, blending, texture arrays and their loaders live in `gl/`. A pass
   is a shader plus a function that writes floats.
5. **Everything that can be tested without a GL context is.** Float layouts,
   draw order, overlay contents, tool behaviour, change detection.

## Principles

- Behaviour does not change. Every phase is verified by the same tests, the
  same `make check`, the debug benchmark, and eyeballing the room. A phase
  that changes what the user sees is a bug in the phase.
- Pure moves first. A phase that moves code does not also reshape it.
- One phase per branch or per commit series. Do not start phase N+1 until
  phase N is green and merged.
- Zero allocation per frame stays a rule. The reused `out` arrays, the
  growable `Float32Array` instance buffers, and the scratch `Point`/`Rect`
  objects are deliberate. New code follows the same pattern.
- The draw call count does not go up. A refactor that splits one batch into
  two stages for tidiness has made the frame slower for nothing.
- Bundle size and frame time are measured after every phase. Both should go
  down or stay flat.
- Exit criteria are greps and counts. Each one was checked against the tree
  before it was written down; a criterion a phase cannot meet is a bug in
  this document, not a reason to skip it.

## Decisions

Things this document was asked to settle, settled.

- **`Tool` and `Modifiers` stay in `render/input.ts`.** The pointer wiring
  owns the contract and the interaction layer imports it downward. There is
  no upward import, type-only or otherwise.
- **The interaction directory is `modes/`, not `tools/`.** `tools.ts` is the
  toolbar, and `fog-tool.ts`, `draw-tool.ts` and `layer-tool.ts` are its
  option panels. A `tools/` directory beside them would give the word four
  meanings. The toolbar chooses a mode; each mode implements `Tool`.
- **The DOM pawn overlay becomes `hud.ts`.** `overlay.ts` today is the
  floating name/HP/AC panel that follows the selected pawn, and it exports an
  `Overlay`. The render struct that phase 4 introduces is the thing the word
  overlay actually means here, so the panel is renamed `Hud` and the struct
  keeps `Overlay`. The `data-pawn-overlay` attribute in templ does not
  change. This is the one edit to a file on the not-touched list.
- **Escape never changes the toolbar.** It drops what the current tool is
  holding and nothing else, which is what it does today. `Tools` has no
  setter and does not get one.
- **Revisions live beside `State`, not inside it.** `reduce.test.ts` compares
  the whole client state to fixtures the Go tests generate, and the Makefile
  says byte-for-byte agreement is the point of having a reducer in Go at all.
  A field the server does not know about breaks every step.
- **Condition rings and outlines are one stage. Over marks and labels are one
  stage.** That is how they are batched today and splitting them adds draw
  calls.
- **Concealment is computed by the pawn stage.** Whether a player can see a
  pawn depends on role, user, the viewed layer and the fog shapes, all of
  which are in the frame. The interaction layer does not supply it.

## Where things are today

### Layout

```
server/js/room/
  main.ts                 wiring; one effect per Renderer method
  protocol.ts             generated from Go, do not edit
  socket.ts store.ts      transport and reducer
  effects.ts              event fan-out, touchesPawns
  pawns.ts                1084 lines: gestures, arming, measuring, previews,
                          hit testing, all overlay production, and its own
                          document keydown listener
  fog.ts draw.ts          fog and drawing interaction, plus their geometry
  fog-tool.ts draw-tool.ts
  layer-tool.ts           DOM option panels and the layer menu; not tools
  tools.ts                which toolbar button is pressed, and held space
  keys.ts                 typing(): whether a key event belongs to a field
  selection.ts handles.ts follow.ts initiative.ts
  overlay.ts              the DOM pawn HUD; not a render overlay
  debug.ts                the benchmark and stress buttons
  window.ts panels.ts dialogs.ts hp.ts color.ts ping-sound.ts exit.ts ulid.ts
  pawn-menu.ts pawn-window.ts initiative-menu.ts layered-menu.ts layer-bar.ts
  render/
    renderer.ts           541 lines: owns 13 passes, camera, draw order,
                          context loss, benchmark, stress, view commands
    gl.ts                 createContext, createProgram, uniforms,
                          fullscreenTriangle
    *-pass.ts             tile, grid, fog, decal, stroke, path, aura, ring, pawn
    sprites.ts glyphs.ts  texture resources
    tiles.ts              pyramid math AND the slot allocator AND the loader
    camera.ts frame.ts input.ts layers.ts
    path.ts scene.ts wounds.ts pings.ts decals.ts stress.ts pyramid.ts
```

### Problems

**`render/renderer.ts` is a god function.** `mountRenderer` owns thirteen pass
instances, camera travel, the benchmark sweep, context loss, clear-colour
probing, view and blood commands, stress pawns, and the entire draw order.
The draw order is a hard-coded sequence inside `drawPawns`:

```
tiles → grid → decals → strokes → [GM fog] → floor marks → auras → pawns
→ rings → ghosts → handles → pings → [player fog] → over marks
```

Three things about that order are easy to misread:

- Fog is one pass drawn once per frame. A GM sees it at the first bracket,
  a player at the second. `drawFog` is called twice and returns early once.
- The ring pass is filled and drawn three times: condition rings and
  selection outlines share one batch, then handles, then pings.
- Floor marks are the ruler cells and nothing else. Over marks are one
  batch holding segments, labels, ruler lines and ruler labels.

Adding any visual means editing that function.

**Seven passes duplicate the same instanced-quad boilerplate.** Measured:

| Repeated in `render/*-pass.ts` | Count |
| --- | --- |
| Corner buffer `[0,0,1,0,0,1,1,1]` | 7 |
| `gl.blendFunc` enable/disable pairs | 9 |
| `clipMatrix(cam, deviceWidth, deviceHeight, dpr, matrix)` | 7 |
| Growable `Float32Array` doubling loop | 7, plus an eighth in `fog-pass.ts` over a vertex array rather than an instance buffer |

Decal, ring and aura passes are structurally identical (14 floats: rect,
colour, style, spin) apart from the fragment shader. Pawn is 20, path 16,
stroke and tile 9. The only pass tests are `aura-pass.test.ts` and
`grid-pass.test.ts`, and both cover pure helpers (`auraTurn`, `auraColor`,
`parseColor`), not a draw. No draw is tested because each pass needs a live
`WebGL2RenderingContext` to be constructed.

**Per-frame values are recomputed per pass.** Every `draw` takes
`(cam, deviceWidth, deviceHeight, dpr)` and rebuilds the clip matrix; grid
and fog also rebuild the inverse. `worldPerCssPixel` and
`worldPerDevicePixel` are computed in the renderer and handed to some passes
and not others. `mapPerPixel` on the `Renderer` interface and `fitZoom` in
the renderer each duplicate a value the camera module already computes.

**Dependencies run both ways across the `render/` boundary.**

Rendering reaching up into interaction:

| File | Imports from |
| --- | --- |
| `render/renderer.ts` | `Table`, `Outline`, `Ruler`, `Label`, `Segment`, `GHOST_ALPHA`, `SELECT_COLOR`, `actorColor` from `pawns.ts`; `Handle`, `HANDLE_HALF` from `handles.ts` |
| `render/fog-pass.ts` | `maskRect`, `rectTriangles`, `triangulate` from `fog.ts` |
| `render/stroke-pass.ts` | `strokeSegments` from `draw.ts` |

The renderer also takes a `Table` at runtime for `tool`, `concealed` and the
seven overlay accessors. The parameter is optional in the signature, but
`main.ts` is the only caller and always passes one, so every `table ?`
branch in the renderer is dead.

Interaction reaching down for things that are not rendering:

| File under `render/` | Actually is | Imported by |
| --- | --- | --- |
| `path.ts` | grid snapping, footprints, hit shapes, distances | pawns, fog, draw, handles, selection, follow |
| `scene.ts` | stack order, acting pawn ids | pawns, selection, follow |
| `wounds.ts` | HP bands, heartbeat curves, blood constants | pawn-pass, decals, aura-pass |
| `grid-pass.ts` `parseColor` | colour parsing | draw |
| `camera.ts` `Point`, `Rect` | geometry types | everything |
| `pawn-pass.ts` `Drawn` | the record of a pawn as drawn | pawns, scene, stress |
| `input.ts` `Tool`, `Modifiers` | the pointer contract | pawns, fog, draw |

**The renderer's input is a bag of typed callbacks.** `Table` exposes
`ghosts`, `outlines`, `rulers`, `marks`, `labels`, `handles`, `inHand` and
`concealed` as separate accessors. The renderer knows which pass each feeds
and decomposes rulers into cells, a line and a label itself.

**The renderer is also an event sink with one method per event.**
`pawnsChanged`, `bloodCleared`, `bloodResync` and `pinged` are each a
`Renderer` method plus an effect in `main.ts` that matches on an event type.
A visual that reacts to a new event edits both.

**`pawns.ts` is the other god module.** Press, drag, shape and marquee
gestures, spawn arming, measuring, ping sending, remote drag previews, hit
testing, and all overlay production. Fog and draw already have
`press/drag/release/secondary/hover/key/abandon` but are routed through
`fogging` and `inking` flags inside the pawn gesture code. The `Tool`
interface in `render/input.ts` exists and has exactly one implementation.
`input.ts` routes pointer events only; `pawns.ts` registers its own document
keydown listener and forwards keys to fog, draw, Escape and Delete itself.

**Change detection is reinvented per consumer.**

| Consumer | Mechanism |
| --- | --- |
| fog pass `sync` | string signature built every frame |
| stroke pass `sync` | string signature built every frame, in a variable called `key` |
| pawn pass | `pawnsDirty` flag set by `touchesPawns()` matching event type names |
| decals `watch` | runs only when the pawn pass rebuilds; borrows its detection |
| sprite cache `begin(rebuilding)` | same flag again; clears the live set only on a pawn rebuild |
| sprite cache | `epoch` counter |
| continuous-frame decision | two hand-written `||` chains in `drawFrame` and `drawPawns` |

The pawn rebuild decision therefore gates three consumers. Any stage split
has to keep them agreeing.

**Keyboard handling is scattered.** Seven modules each register a document
keydown listener and each consult `typing()`: `tools.ts`, `pawns.ts`,
`layer-tool.ts`, `draw-tool.ts`, `initiative.ts`, `initiative-menu.ts`,
`pawn-menu.ts`. Five of them handle Escape independently. Precedence is
whatever order `main.ts` mounted them in. Only the `pawns.ts` listener is in
scope here; the rest is on the backlog.

**Small things.**

- `sprites.ts` keeps a `generated` map that is written and never read.
- `tile-pass.ts` has an `abandon(map)` method that nothing calls. It is the
  only caller of `Loader.abandon` and `mapPrefix`, so those die with it.
- Ghosts are drawn by a second `createPawnPass`, which compiles a second
  program. A second batch on the shared program is enough.
- `SPRITE_EDGE` in `pawn-pass.ts` duplicates `SPRITE_SIZE` in `sprites.ts`.
- `teardown()` and `rebuild()` in the renderer list every pass by hand.
- `pingRings` in the renderer is an adapter from `Pings` to `RingPass` that
  exists only because the ring batch is not a type the pings module can name.
- `Stacked` in `scene.ts` is a `Pick` of `Drawn`, so the stack comparison
  depends on a render type for three fields it could declare itself.
- `createGlyphAtlas` returns `null` when it cannot draw, and the renderer
  threads `atlas?.` through disposal.
- `readClearColor` creates a probe canvas on every theme change.
- `Tools` is five predicates (`panning`, `measuring`, `fogging`, `drawing`,
  `pinging`) where the switch wants one mode.

## Target layout

```
server/js/room/
  main.ts
  protocol.ts
  socket.ts store.ts effects.ts

  model/                  pure. no GL, no DOM, no protocol mutation.
    types.ts              Point, Rect, Rgb
    grid.ts               snapAxis, snapPoint, cellAt, cellCentre, supercover,
                          cellsMoved, feetMoved, feetBetween, distanceLabel
    shape.ts              Sized, Placed, footprintOf, pawnExtents, boundsOf,
                          containsPoint, snapPawn, snapsToGrid, radians, spin,
                          unrotate
    health.ts             bandOf, healthOf, hurt, beats, bleeds, severityOf,
                          heartbeat curves, blood sprite selection
    stack.ts              Stacked, compareStack, actingPawnIds
    color.ts              parseColor, hexColor, actorColor, KIND_COLORS,
                          CONDITION_COLORS, SELECT_COLOR, blood and aura colours
    polygon.ts            triangulate, rectTriangles, insideShape, coveredBy,
                          maskRect, concealed
    stroke.ts             strokeSegments, coneCorners, circleSegments,
                          strokeHit, measure
    overlay.ts            Overlay and its element types (phase 4)

  gl/                     WebGL2 toolkit. knows nothing about the room.
    context.ts            createContext
    program.ts            Program: compile, link, uniform lookup, use, dispose
    buffer.ts             GrowableBuffer: the doubling loop, once
    quads.ts              QuadBatch: instanced unit-quad batch over a declared
                          attribute layout, on a GrowableBuffer
    blend.ts              blended(gl, fn)
    texture-array.ts      TextureArray: storage + Slots (LRU layers)
    loader.ts             Loader: fetch queue, priority, abort, retry, 404 memo
    fullscreen.ts         fullscreenTriangle

  render/
    renderer.ts           context lifecycle, camera, frame loop, stage list
    frame.ts              requestAnimationFrame loop, resize, timings
    frame-context.ts      FrameContext: everything a stage needs, computed once
    camera.ts             camera math, plus fitZoom
    camera-controller.ts  travel, focus, view commands, benchmark sweep
    theme.ts              clear colour probe and the THEME_CHANGE listener
    input.ts              pointer wiring; owns Tool and Modifiers
    layers.ts             layer crossfade (unchanged)
    resources.ts          Resources: sprite array, glyph atlas, tile stores,
                          and the per-frame begin/end bracket
    sprites.ts glyphs.ts tiles.ts pyramid.ts
    decals.ts pings.ts stress.ts
    stages/
      stage.ts            the Stage interface
      tiles.ts grid.ts decals.ts strokes.ts fog.ts floor-marks.ts auras.ts
      pawns.ts rings.ts ghosts.ts handles.ts pings.ts over-marks.ts
    shaders/              one file per program, exporting vertex + fragment
      quad-sprite.ts quad-shape.ts quad-textured.ts stroke.ts grid.ts fog.ts

  modes/                  interaction. each implements Tool from render/input.
    switch.ts             ToolSwitch: routes pointer and key to the active tool
    pan.ts select.ts measure.ts ping.ts place.ts fog.ts draw.ts
    gestures.ts           press/drag/shape/marquee state (from pawns.ts)
    previews.ts           remote drag previews (from pawns.ts)
    hit.ts                hitTest (from pawns.ts)
  hud.ts                  was overlay.ts
  selection.ts handles.ts follow.ts initiative.ts tools.ts keys.ts ...
```

Dependency direction, enforced by review:

```
protocol.ts ← model/ ← gl/ ← render/ ← modes/ ← main.ts
                 ↑                ↑         ↑
                 └── selection, hud, follow, panels, layer-bar, debug
```

`gl/` does not import `model/` today and should not need to. It is listed in
the chain only so the direction is unambiguous. `layer-bar.ts`, `debug.ts`
and `follow.ts` see `render/` only through the `Renderer` interface and a
focus callback.

## Phases

### Phase 1: carve out `model/`

Pure moves. No signature changes, no logic changes. Tests move with their
functions.

- [x] `model/types.ts`: `Point`, `Rect` from `render/camera.ts`. Re-export
      from `camera.ts` for one phase so the diff is only import lines, then
      remove the re-export at the end of the phase. Introduce
      `type Rgb = readonly [number, number, number]` and use it everywhere
      the tuple is spelled out.
- [x] `model/grid.ts` and `model/shape.ts`: split `render/path.ts`. `Sized`,
      `Placed`, `TINY_SCALE` and `PATH_CELLS_MAX` go with their functions.
      `path.ts` is deleted.
- [x] `model/health.ts`: `render/wounds.ts` minus the two blood colours.
      `wounds.ts` is deleted.
- [x] `model/stack.ts`: `compareStack`, `stackRank`, `actingPawnIds` from
      `render/scene.ts`. `Stacked` is declared structurally as
      `{ id: string; kind: Pawn["kind"]; z: number }` because today it is a
      `Pick` of `Drawn`, and `Drawn` is a render type. That is the one
      non-move in this phase. `visiblePawns`, `ringRadius`,
      `CONDITION_RINGS_MAX`, `RING_GAP`, `RING_WIDTH` stay in `render/` and
      move into the pawn and ring stages in phase 3.
- [x] `model/color.ts`: `parseColor` from `grid-pass.ts`; `hexColor`,
      `actorColor`, `ACTOR_COLORS`, `SELF_COLOR`, `SELECT_COLOR` from
      `pawns.ts`; `KIND_COLORS`, `CONDITION_COLORS` from `sprites.ts`;
      `AURA_GOLD`, `auraColor` from `aura-pass.ts`; `BLOOD_FRESH`,
      `BLOOD_DRIED` from `wounds.ts`.
- [x] `model/polygon.ts`: `rectTriangles`, `triangulate`, `signedArea`,
      `isEar`, `cross`, `inTriangle`, `insideShape`, `coveredBy`, `maskRect`,
      `MaskRect` from `fog.ts`, plus `concealed(pawn, shapes, viewed, role,
      user)` lifted from the `covered` and `concealed` closures in
      `createFog`. `fog.ts` keeps `snapCorner`, the options types and
      `createFog`, and its `concealed` method calls the new function.
- [x] `model/stroke.ts`: `strokeSegments`, `coneCorners`, `circleSegments`,
      `strokeHit`, `segmentDistanceSquared`, `measure`, and the `boxOf` cache
      from `draw.ts`. `draw.ts` keeps `createDraw` and its option types.
- [x] Fix `SPRITE_EDGE` in `pawn-pass.ts` to import `SPRITE_SIZE`.
- [x] Delete the unused `generated` map in `sprites.ts`.
- [x] Delete `abandon` from `TilePass`, `abandon` from `Loader`, and
      `mapPrefix` from `tiles.ts`. Nothing outside `tile-pass.ts` calls any
      of them and `tiles.test.ts` does not cover them. If layer switching
      should abort in-flight tiles, that is a feature and goes on the
      backlog, not in this phase.
- [x] Tests. Nine files import from a module this phase moves:
      `render/path.test.ts`, `render/scene.test.ts`, `render/wounds.test.ts`,
      `render/grid-pass.test.ts`, `render/aura-pass.test.ts`,
      `render/tiles.test.ts`, `render/decals.test.ts`, `handles.test.ts`,
      `rules.test.ts`. `grid-pass.test.ts` becomes `model/color.test.ts`.
      `aura-pass.test.ts` splits: the `auraTurn` tests stay, the colour
      tests join `model/color.test.ts`. `rules.test.ts` keeps reading the Go
      fixtures and imports `snapAxis` and `bandOf` from `model/`.

Done when: `grep -rn 'from "\.\./' server/js/room/render/` shows
`../protocol.ts`, `../model/`, the `events.js` import in `renderer.ts`, and
in `renderer.ts` only the `Table`, `Outline`, `Ruler`, `Label`, `Segment`,
`GHOST_ALPHA` imports from `../pawns.ts` and `Handle`, `HANDLE_HALF` from
`../handles.ts`. Those eight leave in phase 4. `fog-pass.ts` and
`stroke-pass.ts` import from `../model/` only. Test count 494, unchanged.
`make check` green.

### Phase 2: the `gl/` toolkit

Introduce the toolkit, then port passes one at a time. Each port is its own
commit and is verified visually before the next.

**`gl/program.ts`**

Mostly a move. `createProgram` and `uniforms<K>` already exist in
`render/gl.ts`; the wrapper adds `use()` and `dispose()` and takes the
uniform names at construction. `gl.ts` splits into `context.ts`,
`program.ts` and `fullscreen.ts` and is deleted.

```ts
export interface Program<K extends string> {
	readonly handle: WebGLProgram;
	readonly at: Record<K, WebGLUniformLocation>;
	use(): void;
	dispose(): void;
}
export function createProgram<K extends string>(
	gl: WebGL2RenderingContext, vertex: string, fragment: string, uniforms: readonly K[],
): Program<K>;
```

**`gl/buffer.ts`**

```ts
export interface GrowableBuffer {
	readonly data: Float32Array;
	reserve(floats: number): void;
	upload(gl: WebGL2RenderingContext, buffer: WebGLBuffer, floats: number): void;
}
```

The doubling loop, once. `QuadBatch` is built on it. The fog mask paint,
which grows a plain vertex array rather than an instance buffer, uses it
directly; that is the eighth loop.

**`gl/quads.ts`**

```ts
export interface Attribute { size: 1 | 2 | 3 | 4 }
export interface QuadBatch {
	readonly data: Float32Array;
	readonly stride: number;
	count: number;
	begin(): void;
	reserve(instances: number): void;
	cursor(): number;
	upload(): void;
	draw(): void;
	dispose(): void;
}
export function createQuadBatch(
	gl: WebGL2RenderingContext, layout: readonly Attribute[], initial: number,
): QuadBatch;
```

`data` is the current backing store and may be replaced by `reserve()`.
`stride` is floats per instance. `begin()` zeroes the count. `cursor()`
returns the index of the next instance and increments the count. `upload()`
is a `bufferData` of the used prefix. `draw()` binds the VAO and issues one
`drawArraysInstanced` over a four-vertex triangle strip.

`cursor()` is the write API: the pass does `const i = batch.cursor();
batch.data[i] = x; ...`. This keeps the hot loop free of per-field calls and
makes the float layout a plain array a test can read back. A `FakeBatch` in
the test helpers implements the same interface over a growable array with no
GL.

The corner buffer is created once per batch. If profiling shows that matters,
share it per context later. It does not matter now.

**`gl/blend.ts`**

```ts
export function blended(gl: WebGL2RenderingContext, draw: () => void): void;
```

Standard `SRC_ALPHA, ONE_MINUS_SRC_ALPHA`. The fog mask paint is the one
unblended draw and calls nothing.

**`gl/texture-array.ts`** and **`gl/loader.ts`**

`Slots` and `Loader` move out of `render/tiles.ts` unchanged. `tiles.ts`
keeps only pyramid and range math. `tiles.test.ts` splits the same way: the
`Slots` and `newLoader` tests become `gl/texture-array.test.ts` and
`gl/loader.test.ts`. `TextureArray` wraps `texStorage3D`, the parameters,
`texSubImage3D` upload into a slot, and `dispose`. Both `sprites.ts` and the
tile stage use it.

**Porting order**

- [ ] `stroke-pass.ts`. Already has the program/batch split; port is
      mechanical and proves the toolkit.
- [ ] `ring-pass.ts`, `aura-pass.ts`, `decal-pass.ts` together. Same 14-float
      layout `[rect(4), color(4), style(4), spin(2)]`. One `SHAPED_QUAD`
      layout constant, three shaders.
- [ ] `path-pass.ts`.
- [ ] `pawn-pass.ts`. Split into program and batch so ghosts become a second
      batch on the same program, not a second program.
- [ ] `tile-pass.ts`. Uses `TextureArray` per tile size.
- [ ] `grid-pass.ts`, `fog-pass.ts`. Fullscreen-triangle passes; the program
      wrapper and `blended` apply, and the fog mask's vertex array moves onto
      `GrowableBuffer`.
- [ ] Shaders move to `render/shaders/*.ts` as exported template strings.
      Constants interpolated into shader source (`BORDER_PIXELS`,
      `AURA_REACH`) move with them.
- [ ] Tests: one per ported pass asserting the float layout written for a
      known input against `FakeBatch`.

Done when: no `*-pass.ts` file contains `createVertexArray`, `blendFunc`, or
a `new Float32Array(size)` growth; `render/gl.ts` no longer exists. Bundle
size recorded and compared to the phase 1 baseline.

### Phase 3: frame context and stages

This is the phase that meets goal 1.

**`render/frame-context.ts`**

```ts
export interface FrameContext {
	gl: WebGL2RenderingContext;
	now: number;
	camera: Camera;
	viewport: Viewport;
	deviceWidth: number;
	deviceHeight: number;
	dpr: number;
	scale: number;
	worldPerCssPixel: number;
	worldPerDevicePixel: number;
	clip: Float32Array;
	clipInverse: Float32Array;
	clear: Rgb;
	role: Role;
	user: string;
	state: State;
	viewed: Layer | null;
	viewedID: string;
	cell: number;
	painted: readonly Painted[];
	rebuild: boolean;
	overlay: Overlay;
	resources: Resources;
}
```

Filled once at the top of `drawFrame`. `camera` is read-only for stages.
`viewport` is CSS pixels. `scale` is `camera.zoom * dpr`. `clip` and
`clipInverse` are computed once from the camera. `cell` is
`max(1, grid.cellSize)`. `painted` is `LayerView.draws()`. `user` is the
viewer's id, which concealment needs. `overlay` is typed as the `Table`
until phase 4. Every pass `draw(cam, w, h, dpr)` becomes `draw(frame)`.

**`rebuild`** is the pawn rebuild decision, made once by the renderer:
`pawnsDirty || cell !== lastCell || resources.spriteEpoch() !== lastEpoch`.
The pawn stage, the decals stage and `Resources.begin` all read it, which is
how the three consumers that share the flag today keep agreeing after the
split. Phase 5 replaces its inputs with revisions.

**`render/stages/stage.ts`**

```ts
export interface Stage {
	build?(frame: FrameContext): void;
	draw(frame: FrameContext): void;
	settling?(frame: FrameContext): boolean;
	event?(event: Event): void;
	dispose(): void;
}
export type StageFactory = (gl: WebGL2RenderingContext, resources: Resources) => Stage;
```

`build` is the CPU side and fills batches. `draw` is the GPU side.
`settling` is true while the stage wants another frame. `event` receives
every socket event the renderer is handed, so a stage that reacts to one
does not need a `Renderer` method.

**`render/resources.ts`**

Owns the sprite `TextureArray` and its loader, the glyph atlas, and the tile
stores. Created once, disposed on context loss, recreated on restore. Stages
receive it at construction and again via the frame. This removes the three
different texture-passing conventions (pawn pass stores the texture at build,
decal pass takes it as a draw argument, path pass takes the atlas at
construction).

It also owns the per-frame bracket. `begin(frame)` does what
`sprites.begin(rebuilding)` and `tiles.begin()` do today; `end(): boolean`
does what `sprites.end()` and `tiles.end()` do and returns whether anything
is still uploading. The glyph atlas is `GlyphAtlas | null` here, as it is
today, and the marks stages draw no labels without it.

**The draw order becomes data**

```ts
export function stagesFor(role: Role): readonly StageFactory[] {
	return [
		tilesStage, gridStage, decalsStage, strokesStage,
		...(role === "gm" ? [fogStage] : []),
		floorMarksStage, aurasStage, pawnsStage, ringsStage, ghostsStage,
		handlesStage, pingsStage,
		...(role === "player" ? [fogStage] : []),
		overMarksStage,
	];
}
```

One fog stage, placed by role. `ringsStage` holds condition rings and
outlines in one batch as today. `overMarksStage` holds segments, labels and
rulers in one batch as today. Sixteen draws today, sixteen after.

**Events reach stages through the renderer**

`Renderer` gains `event(event: Event): void` and loses `pawnsChanged`,
`bloodCleared`, `bloodResync` and `pinged`. `main.ts` has one effect that
forwards every event. The renderer sets `pawnsDirty` from `touchesPawns`
itself until phase 5 deletes both, then hands the event to every stage that
declares `event`. The decals stage handles `stroke.cleared` and `snapshot`;
the pings stage handles `pinged`. The ping sound stays in `main.ts`, keyed
on the same event, because it is not a render concern.

`showBlood`, `stress`, `benchmark`, `focus`, `invalidate`, `onFrame`,
`onSettled`, `toScreen`, `mapPerPixel` and `view` stay. They are settings
and UI, not events.

**`renderer.ts` after this phase**

- context creation and loss handling
- `LayerView`, `Camera`, `Viewport`, `Input`
- `Resources` and `Stage[]`, built by mapping `stagesFor(role)`, torn down
  and rebuilt by looping
- `drawFrame`: sync layers, fill the `FrameContext`, `resources.begin`,
  `for (stage) build`, clear, `for (stage) draw`, `resources.end`,
  `again = stages.some(settling) || uploading || input.dragging() || layers.fading() || camera.moving()`
- the public `Renderer` interface

Everything else in today's `renderer.ts` moves:

- [ ] `mountRenderer(mount, state, role, user, table: Table)`. The parameter
      is required and every `table ?` branch goes.
- [ ] `render/camera-controller.ts`: `target`, `travel`, `advanceTravel`,
      `focus`, `settleMap`, `onViewCommand`, `sweep` and `advanceSweep`.
      Exposes `update(now, map): boolean` (still moving) and the
      focus/benchmark entry points. `fitZoom` moves to `camera.ts`, is
      exported, and `fit` uses it instead of repeating the expression.
      `mapPerPixel` returns the same value `worldPerCssPixel` is computed
      from.
- [ ] `readClearColor` and the `THEME_CHANGE` listener into
      `render/theme.ts`, with one probe canvas reused across calls.
- [ ] Rings: three stage instances over a shared `RingBatch` helper, each
      with its own buffer: rings, handles, pings. The `pingRings` adapter in
      the renderer goes away because the pings stage owns a `RingBatch`.
- [ ] `visiblePawns` and the `drawn`/`synthetic` arrays move into the pawns
      stage. `stress()` on the renderer forwards to it. The stage computes
      concealment with `concealed` from `model/polygon.ts` using
      `frame.state.fog`, `frame.viewed`, `frame.role` and `frame.user`;
      `Table.concealed` is no longer read by the renderer.
- [ ] `decals.watch/build` move into the decals stage, gated on
      `frame.rebuild`; `bloodCleared`, `bloodResync`, `showBlood`, and the
      `ROOM_BLOOD` listener go with it. The stage handles `stroke.cleared`
      and `snapshot` through `event`. The renderer forwards `showBlood`.
- [ ] `pings.add` moves into the pings stage, which handles `pinged` through
      `event` and colours it with `actorColor` from `model/color.ts`.
- [ ] `main.ts`: the four event-matching effects that call the renderer
      collapse to `(event) => renderer?.event(event)`. The `overlay.refresh`
      call that rides on `touchesPawns` today stays as its own effect until
      phase 5.
- [ ] Tests: a `RecordingGL` stub good enough to construct stages, a test
      that `stagesFor` issues draws in the expected order for each role, and
      a test that `settling` is true only while something animates.

The `Renderer` interface otherwise keeps the same methods so `layer-bar.ts`,
`debug.ts`, and `follow.ts` do not change in this phase.

Done when: `renderer.ts` is under 200 lines and contains no `gl.uniform`,
`gl.draw`, `table ?`, or per-pass names outside `stagesFor`. `main.ts` calls
exactly one `Renderer` method from inside `fanOut`. Draw count per frame
unchanged.

### Phase 4: tools per mode, and the overlay

Meets goal 2 and closes the last VTT review item. This phase is what were
phases 4 and 5. They were merged because both rewrite the same sixty call
sites in `pawns.test.ts` that read `outlines`, `rulers`, `ghosts` and the
rest, and doing them as one commit series rewrites those sites once.

Those accessors are already unit-tested directly; the overlay struct does
not add coverage. What it adds is one value in place of eight accessors and
the deletion of the ruler decomposition from the renderer.

**`Tool` in `render/input.ts`, extended**

```ts
export interface Tool {
	press(map: Point, screen: Point, mods: Modifiers): boolean;
	drag(map: Point, screen: Point, mods: Modifiers): void;
	release(map: Point, screen: Point, mods: Modifiers): void;
	cancel(): void;
	secondary(map: Point, screen: Point): boolean;
	hover(map: Point | null): void;
	key(e: KeyboardEvent): boolean;
	abandon(): boolean;
	active(): boolean;
	contribute(out: Overlay): void;
	enter?(): void;
	leave?(): void;
}
```

`secondary` returns `void` today and becomes `boolean`. `abandon` is
Escape and returns true if there was something to drop. `enter` and `leave`
bracket being the current tool; `leave` must abandon. `input.ts` itself
calls only the pointer methods and does not change.

**`model/overlay.ts`**

```ts
export interface Overlay {
	ghosts: Drawn[];
	outlines: Outline[];
	segments: Segment[];
	cells: Cell[];
	labels: Label[];
	handles: Handle[];
	inHand: Stroke | null;
	reset(): void;
}
```

`segments` was `marks`. `cells` is `{ x, y, size, color, alpha }` and is
what floor marks draw. `reset()` sets every length to 0 and `inHand` to
null. `Drawn`, `Outline`, `Segment`, `Label`, `Handle` move here from
`pawn-pass.ts`, `pawns.ts`, and `handles.ts`. `Ruler` is deleted: the
measure tool decomposes a ruler into `cells`, one `segment`, and one `label`
itself, which is what the renderer does today on its behalf. There is no
`concealed` here; phase 3 moved that into the pawn stage.

**`tools.ts` gains a mode**

```ts
export type Mode = "select" | "pan" | "measure" | "fog" | "draw" | "ping";
```

`mode()` returns `"pan"` while space is held, otherwise the chosen button.
The five predicates stay until their last callers go in this phase, then are
deleted.

**`modes/switch.ts`**

Holds a map from `Mode` to `Tool`. Implements `Tool` itself so `input.ts`
does not change: every pointer call forwards to the current tool, and
`contribute` concatenates every tool's `contribute`. On mode change it calls
`leave()` on the old and `enter()` on the new.

The switch owns the document keydown listener that `pawns.ts` owns today, in
the same order: `typing()` guard, then the current tool's `key(e)`, then
Escape to `abandon()`, then Delete. Delete is a selection action, not a
tool's; the switch takes a `remove` callback for it as `Table` does now.
Escape does not change the toolbar.

**Tools**

| Tool | From | Takes |
| --- | --- | --- |
| `pan.ts` | the `deps.panning()` early return; press returns false so input pans | nothing |
| `select.ts` | press/drag/shape/marquee gestures, double-click, hover, selection, handles. The bulk of `pawns.ts` | `selection`, `send`, `details`, `menu`, `scale` |
| `measure.ts` | `measured`, `aim`, `measurement()` | `grid`, `scale` |
| `ping.ts` | the ping branch in `press` | `send` |
| `place.ts` | `armed`, `arm`, `place`, `armedShape`, the armed ghost | `send`, `grid` |
| `fog.ts` | `createFog` with `contribute` | `options`, `send`, `grid` |
| `draw.ts` | `createDraw` with `contribute` | `options`, `send`, `scale` |

Shared pieces come out of `pawns.ts` into their own files: `gestures.ts`
(the gesture union and its transitions), `previews.ts` (remote drag previews
and `expirePreviews`), `hit.ts` (`hitTest`). `marqueeSelect` is already in
`selection.ts` and stays there. `Table` survives as the thing `main.ts`
wires and the switch's owner, holding `selection`, `preview(event)`,
`floorChanged`, `arm`, `bounds`, `focus`, `onChange`, and `tool`, which is
the switch.

`fogging` and `inking` flags are deleted. The Escape chain in `abandon()` is
deleted. `deps.fog?.` and `deps.draw?.` null-chasing is deleted.

- [ ] `tools.ts` gains `Mode` and `mode()`.
- [ ] Land `modes/switch.ts` with a single `select` tool that is today's
      `pawns.ts` verbatim, and move the keydown listener into the switch.
      Prove the switch is transparent: `pawns.test.ts` changes only in how
      it constructs the thing under test.
- [ ] Extract pan, ping, measure, place. Each is small.
- [ ] Move fog and draw under `modes/`. Each gains `contribute` and loses
      `outline`, `marks`, `labels`, `inHand`.
- [ ] Rename `overlay.ts` to `hud.ts`, `Overlay` to `Hud`, `mountOverlay`
      to `mountHud`, `overlay.test.ts` to `hud.test.ts`.
- [ ] Add `Overlay` and `contribute` on select. The renderer takes the switch
      as its `Tool` and fills `frame.overlay` from `overlay.reset()` then
      `tool.contribute(overlay)` each frame. Delete the eight `Table`
      accessors and `Ruler`. `FrameContext.overlay` is typed `Overlay`.
- [ ] Split what remains of `pawns.ts` into `gestures.ts`, `previews.ts`,
      `hit.ts`, `select.ts`.
- [ ] `pawns.test.ts` (1461 lines) splits along the same lines. The tests for
      a drag in progress, a marquee, a measure, an armed spawn, and a remote
      preview assert on what `contribute(out)` wrote.
- [ ] Delete the five `Tools` predicates.

Done when: no file under `modes/` is over 400 lines; `pawns.ts` no longer
exists; `render/` imports nothing from `modes/` or `handles.ts`;
`grep -rn 'keydown' server/js/room/modes` shows only `switch.ts`;
`grep -n 'panning()\|measuring()\|fogging()\|drawing()\|pinging()' server/js/room/*.ts`
returns nothing.

### Phase 5: revisions beside the store

Replace string signatures and event-name heuristics with integers, without
touching `State`.

```ts
export interface Revisions {
	pawns: number;
	fog: number;
	strokes: number;
	table: number;
	initiative: number;
}
export function revisions(): Revisions;
export function revise(rev: Revisions, event: Event): void;
```

`revise` bumps the slices an event type touches. `snapshot` bumps all.
`reduce` and `normalize` do not change, so `reduce.test.ts` and the Go
fixtures do not change. `main.ts` calls `revise` in the same effect as
`reduce`, and the renderer takes the `Revisions` object at mount.

- [ ] `store.ts`: `Revisions`, `revisions()`, `revise`.
- [ ] Fog stage: `if (rev.fog === lastRev && viewedID === lastLayer && ...) return`.
      The incremental paint-from-index logic stays; only the "did anything
      change" check moves to the counter.
- [ ] Stroke stage: same, on `rev.strokes`.
- [ ] `frame.rebuild` becomes `rev.pawns`, `rev.table`, `rev.fog` (for
      concealment) and the sprite epoch against their last values. Delete
      `pawnsDirty`, `touchesPawns` in `effects.ts`, and the last event-name
      match in the renderer. The `hud.refresh` effect in `main.ts` keys on
      the same revisions.
- [ ] Tests for `revise`: each event type bumps exactly the slices it should.

Done when: `grep -rn 'signature\|pawnsDirty\|touchesPawns\|let key = ""' server/js/room --include='*.ts'`
returns only the `signature` in `layer-bar.ts`, which is a DOM concern and
not touched.

## Verification, every phase

```sh
make check
node --test ./server/js/room/*.test.ts ./server/js/room/render/*.test.ts ./server/js/room/modes/*.test.ts ./server/js/room/model/*.test.ts ./server/js/room/gl/*.test.ts
```

Update the `Makefile` test glob as directories appear. A glob for a
directory that does not exist yet fails the run, so add each one in the
phase that creates it.

Bundle size, before and after:

```sh
make js && stat -c %s server/public/static/room.js
```

Frame time: open a room with a tiled map, open the debug panel, run the
benchmark, and record `avg` and `p95` in the table below. Run the stress
button (500 pawns) and record again.

| Phase | tests | room.js bytes | bench avg | bench p95 | stress avg | stress p95 |
| --- | --- | --- | --- | --- | --- | --- |
| baseline | 494 | 188120 | | | | |
| 1 | 494 | 177068 | | | | |
| 2 | | | | | | |
| 3 | | | | | | |
| 4 | | | | | | |
| 5 | | | | | | |

Visual check after every phase, as GM and as player: map and grid, a layer
switch with crossfade, fog reveal and hide with prefill, a pawn hidden under
player fog, drag a pawn with riders, resize and rotate an object, condition
rings, an active-turn aura, damage a pawn to bloody and to dead, pen and
shape strokes, erase, ping, measure, a remote drag preview from a second
tab, context loss via `WEBGL_lose_context` in devtools.

## Not touched

These are correct, tested, and small. Leave them alone unless a phase
forces a signature change.

- `render/camera.ts` (gains `fitZoom` in phase 3, nothing else),
  `render/frame.ts`, `render/input.ts` (the `Tool` interface grows in phase
  4; the pointer maths do not change)
- `render/layers.ts`, `render/pyramid.ts`, `render/pings.ts`,
  `render/decals.ts`, `render/stress.ts`
- `socket.ts`, `store.ts` (gains `revise` in phase 5; `reduce` does not
  change), `protocol.ts` (generated)
- `window.ts`, `panels.ts`, `dialogs.ts`, `hp.ts`, `color.ts`, `keys.ts`,
  `ping-sound.ts`, `exit.ts`, `ulid.ts`, the menus, the layer bar, the
  option panels, initiative, follow, debug
- `hud.ts` after its rename in phase 4

## Backlog surfaced by the review

Not part of this refactor. Recorded so they are not lost.

- Abort in-flight tile fetches on layer switch (the deleted `abandon`).
- Regenerate initials and glyph sprites after LRU eviction (the deleted
  `generated` map suggests this was intended).
- One keyboard dispatcher with an explicit precedence. Seven modules
  register their own document keydown listener and five of them handle
  Escape; which one wins is the order `main.ts` mounted them in.
- `fog-tool.ts`, `draw-tool.ts` and `layer-tool.ts` are option panels and a
  layer menu, not tools. Rename them when something else has to touch them.
- Concealment is a linear scan over the viewed layer's fog shapes per pawn
  per rebuild. Fine at today's counts; index it if fog shapes ever number in
  the hundreds.
- A uniform buffer object for the per-frame clip matrix and scale, if
  per-program uniform uploads ever show in a profile. They do not today.
