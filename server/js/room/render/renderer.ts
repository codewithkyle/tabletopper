// The renderer: it owns the canvas, the camera and the passes, and it is the
// only module here that knows what a room is.
//
// IT READS THE STORE AND NEVER WRITES IT. main.ts reduces an event into the
// state and then tells this to draw again; there is one copy of the room on the
// client and it belongs to the reducer. Everything below is a function of that
// state plus a camera, which is why nothing here has to be kept in step with
// anything -- there is no second copy to drift.
//
// THE ORDER OF THE PASSES IS THE ORDER OF THE TABLE. Tiles first, because they
// are the ground; the grid over them, because it is drawn on the ground. Fog,
// strokes and pawns are later phases and each is one more call in this list,
// against the same camera matrix.

import { ROOM_BLOOD, ROOM_VIEW, THEME_CHANGE } from "../../../public/js/events.js";
import type { Camera, Point, Rect, Viewport } from "./camera.ts";
import type { MapRef, Role, State } from "../protocol.ts";
import type { Drawn } from "./pawn-pass.ts";
import type { Segment } from "../pawns.ts";
import type { LayerView } from "./layers.ts";
import { clampToMap, fit, focusTarget, newCamera, zoomAt, zoomTo } from "./camera.ts";
import { createContext } from "./gl.ts";
import { createGlyphAtlas } from "./glyphs.ts";
import { createGridPass } from "./grid-pass.ts";
import { createFogPass } from "./fog-pass.ts";
import { createDecalPass } from "./decal-pass.ts";
import { createPathPass } from "./path-pass.ts";
import { AURA_DISC, AURA_RECT, auraColor, auraTurn, createAuraPass } from "./aura-pass.ts";
import { HIDDEN_ALPHA, createPawnPass } from "./pawn-pass.ts";
import type { PawnPulse } from "./pawn-pass.ts";
import { createRingPass, RING_ELLIPSE, RING_RECT } from "./ring-pass.ts";
import { createStrokePass } from "./stroke-pass.ts";
import { createSpriteCache, CONDITION_COLORS } from "./sprites.ts";
import { createTilePass } from "./tile-pass.ts";
import { newDecals } from "./decals.ts";
import { newPings } from "./pings.ts";
import type { RingTarget } from "./pings.ts";
import { newLayerView } from "./layers.ts";
import { startFrames } from "./frame.ts";
import { pawnExtents } from "./path.ts";
import { CONDITION_RINGS_MAX, RING_WIDTH, actingPawnIds, ringRadius, visiblePawns } from "./scene.ts";
import { cellCentre } from "./path.ts";
import { fastBeat, healthOf, slowBeat } from "./wounds.ts";
import { stressPawns } from "./stress.ts";
import type { Label, Outline, Ruler, Table } from "../pawns.ts";
import type { Handle } from "../handles.ts";
import { HANDLE_HALF } from "../handles.ts";
import { GHOST_ALPHA, SELECT_COLOR, actorColor } from "../pawns.ts";
import { apply, wireInput } from "./input.ts";
import { screenToWorld, worldToScreen } from "./camera.ts";

// HANDLE_WIDTH is how heavy a resize control's line is, and it is heavier than
// the outline's because a five-pixel box drawn at a hairline is a smudge.
//
// THE COLOUR IS THE SELECTION'S, IMPORTED RATHER THAN COPIED. A handle is part
// of the frame round a selected pawn, so the set has to read as one thing, and
// the two constants drifted apart the moment one of them was retuned. See
// SELECT_COLOR in pawns.ts.
const HANDLE_WIDTH = 2;

// VIEW_ZOOM_STEP is what the View menu's Zoom in and Zoom out move by. It is
// larger than a wheel notch because a menu item is a deliberate act and
// reaching the menu again for a second helping is expensive.
const VIEW_ZOOM_STEP = 1.5;

// FOCUS_MS is how long the camera takes to travel to whoever is acting.
//
// IT IS A MOVE AND NOT A CUT. A camera that teleports on every turn leaves the
// viewer working out where on the map they have been put, once per creature,
// for the whole fight; a third of a second of travel answers that question for
// free, because the direction the table slid IS the answer. It is short enough
// that a GM stepping quickly through a row of goblins is never waiting on it,
// and a hand on the table cancels it outright -- see drawFrame.
const FOCUS_MS = 320;

// BENCHMARK_MS is the sweep's length. Ten seconds is long enough to fill the
// tile cache, cross a level boundary in both directions, and give the ring of
// frame samples something to be a 95th percentile of.
const BENCHMARK_MS = 10_000;

export interface Benchmark {
	average: number;
	p95: number;
	frames: number;
	tiles: number;
}

export interface Renderer {
	// invalidate asks for a frame. main.ts calls it when the reducer has
	// changed something the canvas draws.
	invalidate(): void;

	// view is the GM's floor selector, which the menu bar drives.
	view: LayerView;

	// benchmark sweeps the camera for ten seconds and reports what it cost.
	// The callback runs once, at the end.
	benchmark(report: (result: Benchmark) => void): void;

	// pawnsChanged says the table's contents moved, so the instance buffer has
	// to be built again. The renderer cannot notice this on its own: the
	// reducer mutates the store in place, which is what keeps a pawn moving
	// from allocating, and an in-place mutation has nothing to subscribe to.
	pawnsChanged(): void;

	// focus moves the camera onto a box of the table, easing rather than
	// jumping, and gives up the moment a hand touches the canvas.
	//
	// THE CALLER DECIDES WHAT IS WORTH LOOKING AT AND THIS DECIDES NOTHING.
	// What arrives is a rectangle in map pixels; the turn order is what turned
	// the acting line into one, and it is the only caller today. See follow.ts.
	focus(rect: Rect): void;

	// bloodCleared drops one floor's blood.
	//
	// IT IS WIRED TO stroke.cleared, which is what a GM's "wipe the drawing"
	// sends and what TableClear sends for every floor. Clearing a floor is one
	// gesture at a table, and packing the table away at the end of an evening
	// should not leave the next encounter starting on last week's blood.
	bloodCleared(layerID: string): void;

	// bloodResync forgets what every pawn's hit points were, without dropping
	// any of the blood already on the floor.
	//
	// IT IS WIRED TO THE SNAPSHOT. A tab that was asleep or offline through
	// three rounds comes back to a table where a monster is forty points down,
	// and that difference is not a hit -- it is everything that happened while
	// nobody was watching. See resync in decals.ts.
	bloodResync(): void;

	// pinged is somebody pointing at a square, straight off the wire and
	// without a reducer in between: a ping is transient, so there is no state
	// for one to change and nothing to restore on a reconnect.
	//
	// THE PINGER'S OWN COMES THROUGH HERE TOO. See add in pings.ts.
	pinged(layerID: string, x: number, y: number, by: string): void;

	// showBlood is the account setting: whether this viewer's floor is marked
	// at all. main.ts reads it off the page on load and off the settings dialog
	// every time it is saved.
	//
	// TURNING IT OFF CLEANS WHAT IS ALREADY DOWN, which is decals' decision and
	// not this one -- see show in decals.ts. What this owes it is the frame:
	// nothing about the table changed, so nobody else is going to ask for one,
	// and without it the floor stays bloody until the next pan.
	showBlood(on: boolean): void;

	// stress adds synthetic pawns beside the real ones, for the benchmark.
	// Nothing about them is sent anywhere; see stress.ts.
	stress(count: number): number;

	// toScreen is the camera's projection, for the one thing outside this
	// module that needs it: the DOM overlay, which follows a pawn in CSS pixels
	// while everything else on the table is drawn in map pixels.
	toScreen(x: number, y: number, out: Point): Point;

	// mapPerPixel is how much of the table one CSS pixel covers. The table's
	// resize handles are a fixed size on screen and are therefore a moving size
	// on the map, and the hit test that grabs one is the tool's rather than the
	// renderer's -- so the one number it needs crosses here.
	mapPerPixel(): number;

	// onFrame runs after every frame this renderer draws.
	//
	// IT IS FOR ONE CALLER AND ONE WRITE. The DOM overlay follows a pawn while
	// the camera moves, which is a transform per frame -- and the third
	// performance rule bans DOM work in the frame loop for a reason this does
	// not run into: a transform is composited rather than laid out, nothing is
	// READ back, and it happens only while something is hovered or selected.
	// Anything that wanted to measure an element from here would be the rule
	// the note is actually about.
	onFrame(fn: () => void): void;

	// onSettled fires when the viewed layer, or whether it is the active one,
	// has changed -- after the frame that worked it out. The menu bar's floor
	// control follows it rather than reading the store, because the override
	// that makes the two differ lives in here and nowhere else; the table
	// follows it too, to drop a selection made on the floor just left.
	onSettled(fn: () => void): void;

	stop(): void;
}

// mountRenderer answers null when there is nothing to draw on, which is a
// browser without WebGL2 or a page that rendered no canvas. Neither is an
// error: the room's menus, windows and player list are ordinary HTML and go on
// working, and the caller has nothing to do about it.
export function mountRenderer(mount: HTMLElement, state: State, role: Role, table?: Table): Renderer | null {
	const found = mount.querySelector("[data-tabletop-canvas]");
	if (!(found instanceof HTMLCanvasElement)) {
		return null;
	}

	// The two re-declarations are what carry the narrowing above into the
	// closures below: TypeScript does not keep a control-flow narrowing across
	// a function boundary, and every frame is a function boundary.
	const canvas: HTMLCanvasElement = found;

	const context = createContext(canvas);
	if (!context) {
		// The message is a hidden element the page rendered, not markup built
		// here: server/js is not a Tailwind source, so a class name written in
		// this file would never be emitted into the stylesheet.
		mount.querySelector("[data-tabletop-unsupported]")?.removeAttribute("hidden");
		canvas.hidden = true;

		return null;
	}

	const gl: WebGL2RenderingContext = context;

	// EVERYTHING ON THE GPU IS BUILT BY ONE FUNCTION THAT CAN RUN TWICE. A
	// four-hour session on a laptop loses its context -- GPU power switching, a
	// driver reset, the browser reclaiming a backgrounded tab's memory -- and
	// after that every gl call is a silent no-op: the canvas freezes on its
	// last frame or goes black and nothing says why. So the passes are `let`
	// bindings that rebuild() reassigns when the browser hands the context
	// back, and the frame loop sits out the gap. See onContextLost below.
	let grid = createGridPass(gl);
	let tiles = createTilePass(gl, () => frames.invalidate());
	let sprites = createSpriteCache(gl, () => frames.invalidate());
	let pawnPass = createPawnPass(gl);

	// A SECOND PAWN PASS FOR THE GHOSTS, because they are the one thing on the
	// table that changes every frame. The committed pawns are rebuilt on a
	// change and the previews are rebuilt continuously; sharing one buffer
	// would mean rebuilding the whole table at the rate of a hand.
	let ghostPass = createPawnPass(gl);
	let rings = createRingPass(gl);

	// THE AURA IS THE RING PASS'S OPPOSITE NUMBER AND SO IT IS ITS OWN PASS. A
	// condition is a hairline drawn OVER a creature and an aura is a soft band
	// drawn UNDER one, which is two draws either side of the pawns whatever else
	// is true -- and the shader has nothing in common with the outline's. See
	// aura-pass.ts.
	let auras = createAuraPass(gl);
	let atlas = createGlyphAtlas(gl);

	// THE BLOOD IS A PASS AND A POOL. The pass draws tinted quads off the sprite
	// cache; the pool decides what is on the floor and how dry it is, and it is
	// the only thing here that keeps state the store does not -- which is also
	// why it is the one thing here a lost context does not take with it. See
	// decals.ts.
	let decalPass = createDecalPass(gl);
	const decals = newDecals();

	// AND THE PINGS ARE A POOL WITH NO PASS OF THEIR OWN. A ping is a hollow
	// circle, which the ring pass already draws three times a frame for things
	// that are not pings, so what this adds is a fourth batch through it rather
	// than a fifth program. The pool is the other thing in here the store does
	// not keep -- a ping is transient on the server too -- and a lost context
	// leaves it alone for decals' reason.
	const pings = newPings();

	// The adapter, allocated once. It closes over `rings` rather than capturing
	// it, which is what keeps it working across a context restore: that
	// reassigns the pass, and this reads the binding at the moment it draws.
	const pingRings: RingTarget = {
		ellipse(x, y, radius, color, alpha, thickness) {
			rings.add(x, y, radius, radius, color, alpha, thickness, RING_ELLIPSE);
		},
	};

	// TWO PATH PASSES, UNDER AND OVER. The highlighted cells are the floor and
	// go beneath the pawns standing on them; the line and the distance label
	// are the ruler and go on top. See path-pass.ts.
	let floorMarks = createPathPass(gl, atlas);
	let overMarks = createPathPass(gl, atlas);

	// THE FOG IS A MASK AND A COVER, and the cover is the grid pass's own shape:
	// one full-viewport triangle over an infinite table rather than a quad the
	// size of the map. What it is drawn AFTER depends on who is looking; see
	// drawPawns. It comes back empty on a restored context like every other
	// cache here, and the next frame rasterises the floor's shapes again.
	let fog = createFogPass(gl);

	// THE DRAWING IS TWO BUFFERS AND ONE PROGRAM, and the finished one is
	// rebuilt on a change rather than per frame -- which is the same bargain the
	// pawn buffer makes and for the same reason. See stroke-pass.ts.
	let strokes = createStrokePass(gl);

	// teardown frees every GPU resource the passes hold, and rebuild makes
	// them again on a context the browser has restored. Both caches come back
	// empty -- the tiles and the pictures refetch -- and the pawn buffer is
	// marked stale so the next frame builds it against the new sprite cache.
	function teardown(): void {
		tiles.dispose();
		grid.dispose();
		pawnPass.dispose();
		ghostPass.dispose();
		decalPass.dispose();
		rings.dispose();
		auras.dispose();
		floorMarks.dispose();
		overMarks.dispose();
		fog.dispose();
		strokes.dispose();
		sprites.dispose();
		atlas?.dispose();
	}

	function rebuild(): void {
		grid = createGridPass(gl);
		tiles = createTilePass(gl, () => frames.invalidate());
		sprites = createSpriteCache(gl, () => frames.invalidate());
		pawnPass = createPawnPass(gl);
		ghostPass = createPawnPass(gl);
		rings = createRingPass(gl);
		auras = createAuraPass(gl);
		atlas = createGlyphAtlas(gl);
		decalPass = createDecalPass(gl);
		floorMarks = createPathPass(gl, atlas);
		overMarks = createPathPass(gl, atlas);
		fog = createFogPass(gl);
		strokes = createStrokePass(gl);

		pawnsDirty = true;
		lastEpoch = -1;
	}

	// lost is whether the context is gone, and while it is the frame loop
	// draws nothing: a frame drawn into a lost context is a frame's worth of
	// no-ops, and the loop stopping is what lets the restore start it again.
	let lost = false;
	const restoring = mount.querySelector("[data-tabletop-restoring]");

	// preventDefault IS THE WHOLE OF THE FIRST HALF. Without it the browser
	// does not try to restore the context at all; with it, webglcontextrestored
	// follows once the GPU is back, which is the second half.
	function onContextLost(e: Event): void {
		e.preventDefault();
		lost = true;
		if (restoring instanceof HTMLElement) {
			restoring.hidden = false;
		}
	}

	// THE OLD PASSES ARE NOT DISPOSED ON RESTORE. Their handles belonged to the
	// lost context and are already gone; deleting them would be more calls
	// into nothing. They are dropped and built again.
	function onContextRestored(): void {
		rebuild();
		lost = false;
		if (restoring instanceof HTMLElement) {
			restoring.hidden = true;
		}
		readClearColor();
	}

	canvas.addEventListener("webglcontextlost", onContextLost);
	canvas.addEventListener("webglcontextrestored", onContextRestored);

	const layers = newLayerView(mount.dataset.role === "gm");

	const camera: Camera = newCamera();
	const viewport: Viewport = { width: 1, height: 1 };
	let dpr = window.devicePixelRatio || 1;

	const clear = { r: 0, g: 0, b: 0 };

	// lastMap is how "the map changed" is noticed without a subscription. The
	// key is the asset and its tiling generation, so a re-tiled map counts as a
	// new one -- which it is, at new URLs, with possibly new dimensions.
	let lastMap = "";
	let lastWidth = 0;
	let lastHeight = 0;

	let sweep: { from: number; report: (result: Benchmark) => void; tiles: number } | null = null;

	// THE FOLLOWED TURN, MID-TRAVEL. travel is where the camera started and
	// when; target is where it is going, and it is a whole camera rather than a
	// point because the zoom may have to give as well as the position.
	//
	// BOTH ARE ALLOCATED ONCE, like everything else this loop touches. A turn
	// change is not a hot path, but a Camera allocated per turn is a Camera
	// allocated in the same file that reuses a matrix, a rect and two anchors,
	// and the inconsistency is the part that would get copied.
	const target: Camera = newCamera();
	let travel: { x: number; y: number; zoom: number; from: number } | null = null;

	const settled: (() => void)[] = [];
	let framed: (() => void) | null = null;
	let lastViewed = "";
	let lastFollowing = true;

	// The pawn instance buffer is rebuilt on a change rather than per frame,
	// and these are every way it can change: the reducer said so, the floor
	// moved, the cell size moved, or a picture landed and with it the aspect
	// ratio a quad is fitted to.
	// THE PULSE IS READ ONCE A FRAME FOR THE WHOLE TABLE. It is a uniform rather
	// than a per-instance value because the pawn buffer is rebuilt when the
	// table changes and never per frame; see pawn-pass.ts. The object is reused
	// for the reason everything else in this loop is.
	const pulse: PawnPulse = { slow: 0, heart: 0 };

	// THE TWO ALPHAS ARE THE WHOLE DIFFERENCE BETWEEN THE TWO ROLES. A player's
	// cover is opaque, so a hidden floor is the exact shade of the desk it sits
	// on and nothing on their screen says where the map ends. The GM's is half,
	// which is the old client's mix and reads as "hidden from them" rather than
	// as damage to the picture.
	const PLAYER_FOG_ALPHA = 1;
	const GM_FOG_ALPHA = 0.5;

	const drawn: Drawn[] = [];
	let synthetic: Drawn[] = [];
	let pawnsDirty = true;
	let lastCell = 0;
	let lastEpoch = -1;

	// The tool is given the pointer in MAP pixels, which is a question only the
	// camera can answer -- so the conversion crosses as a callback and input.ts
	// goes on knowing nothing about a camera.
	const input = wireInput(
		canvas,
		() => frames.invalidate(),
		(x, y, out) => screenToWorld(camera, viewport, x, y, out),
		table?.tool ?? null,
	);

	// Per-frame scratch for what the table draws over the pawns. They are
	// arrays the callee fills rather than returns, because they are read every
	// frame for as long as a hand is moving.
	const ghosts: Drawn[] = [];
	const outlines: Outline[] = [];
	const rulers: Ruler[] = [];
	const handles: Handle[] = [];
	const segments: Segment[] = [];
	const labels: Label[] = [];

	const frames = startFrames({
		mount,
		canvas,
		render: drawFrame,
		resized() {
			// THE ONLY DOM READ IN THE WHOLE LOOP, and it is here rather than
			// in drawFrame because a getBoundingClientRect inside a frame
			// forces layout on every frame. Size changes are rare; frames are
			// not.
			dpr = window.devicePixelRatio || 1;
			const rect = mount.getBoundingClientRect();
			viewport.width = Math.max(1, rect.width);
			viewport.height = Math.max(1, rect.height);
		},
	});

	readClearColor();
	window.addEventListener(THEME_CHANGE, readClearColor);
	window.addEventListener(ROOM_VIEW, onViewCommand as EventListener);
	window.addEventListener(ROOM_BLOOD, onBloodCommand);

	frames.invalidate();

	function drawFrame(): boolean {
		if (lost) {
			return false;
		}

		const now = performance.now();

		layers.update(state.table, now);

		const viewedID = layers.viewed()?.id ?? "";
		const following = layers.following();
		if (viewedID !== lastViewed || following !== lastFollowing) {
			lastViewed = viewedID;
			lastFollowing = following;
			for (const fn of settled) {
				fn();
			}
		}

		const painted = layers.draws();
		const map = layers.viewed()?.map ?? null;

		settleMap(map);

		const sweeping = advanceSweep(now, map);
		if (!sweeping) {
			// A HAND ON THE TABLE OUTRANKS THE TURN ORDER. A pointer that is
			// down -- panning, holding a token, dragging a marquee -- has said
			// where this person wants to look more recently than the tracker
			// did, and a camera that slid out from under it would read as the
			// drag itself being broken. It is dropped and not deferred: by the
			// time the hand comes off, "go to whoever is up" is stale.
			if (input.dragging()) {
				travel = null;
			}

			if (apply(input.pending, camera, viewport)) {
				if (map) {
					clampToMap(camera, viewport, map.width, map.height);
				}
			} else {
				advanceTravel(now);
			}
		}

		// THE MASK IS BROUGHT UP TO DATE BEFORE THE FRAME AND NOT DURING IT,
		// because syncing binds another framebuffer and another viewport. Doing
		// it here means the two lines below put both back, rather than every
		// pass after the fog having to distrust what it was handed.
		//
		// A floor with its fog off is not synced at all. Most rooms have no fog
		// on any floor, and the cost of the feature for them is this branch.
		const fogged = layers.viewed();
		if (fogged?.fogEnabled) {
			fog.sync(state.fog, fogged.id, map, fogged.fogPrefill, state.table.grid.cellSize);
		}

		gl.viewport(0, 0, canvas.width, canvas.height);
		gl.clearColor(clear.r, clear.g, clear.b, 1);
		gl.clear(gl.COLOR_BUFFER_BIT);

		tiles.begin();
		for (const layer of painted) {
			tiles.draw(camera, layer.map, layer.alpha, canvas.width, canvas.height, dpr);
		}
		const uploading = tiles.end();

		grid.draw(camera, state.table.grid, canvas.width, canvas.height, dpr);

		const loading = drawPawns(viewedID, now);

		framed?.();

		// The reasons to draw again, and the loop stops when none of them
		// holds: a button or a finger is down, the crossfade is partway
		// through, tiles or pictures are queued for upload, or the benchmark is
		// driving. drawPawns folds in three more of its own -- blood that is
		// still drying, a creature with a heartbeat, and somebody's turn -- and
		// only the last two of those have no end in sight. See the note beside
		// them.
		return input.dragging() || layers.fading() || uploading || loading || sweeping || travel !== null;
	}

	// drawPawns is the table's contents: the blood and the highlighted cells
	// under them, the pawns themselves, and the rings round them.
	//
	// THE ORDER IS THE ORDER OF THE TABLE, as it is for the tiles and the grid.
	// Blood is the floor itself and goes under everything; a highlight is on the
	// floor and goes under the pawn standing on it; the aura round whoever is
	// acting is the light they are standing in and goes under them too; a
	// condition ring is on the pawn and goes over it; the ruler's line and its
	// distance are read against everything else and go last.
	//
	// A WOUND IS IN NONE OF THOSE LAYERS, because it is not a layer: it is drawn
	// INTO the pawn by the pawn's own shader. See wounds.ts -- what is outside a
	// pawn is its conditions, and health is not one of them.
	function drawPawns(viewedID: string, now: number): boolean {
		const tableGrid = state.table.grid;
		const cell = Math.max(1, tableGrid.cellSize);

		// Every way the instance buffer can go stale: the reducer said so, the
		// floor moved, the cell size moved, or a picture landed and with it the
		// aspect ratio a quad is fitted to.
		const rebuilding = pawnsDirty || cell !== lastCell || sprites.epoch() !== lastEpoch;

		sprites.begin(rebuilding);

		// The two screen-to-table scales, and they are two because the things
		// they size want different units. A ring is a hairline and is measured
		// in the pixels that actually exist; a label is text and is measured in
		// the ones a person reads. See scene.ts.
		const worldPerDevicePixel = 1 / Math.max(camera.zoom * dpr, 1e-4);
		const worldPerCssPixel = 1 / Math.max(camera.zoom, 1e-4);

		floorMarks.begin(worldPerCssPixel);
		overMarks.begin(worldPerCssPixel);

		if (rebuilding) {
			// A PAWN UNDER THE COVER IS NOT DRAWN AT ALL for a player, rather
			// than drawn and then painted over. Painting over would work for
			// the picture and not for the rest of it: a concealed creature
			// would still be in the buffer, still be labelled on hover and
			// still be swept up by a marquee. One filter, asked in all four
			// places; see pawns.ts.
			visiblePawns(state.pawns, viewedID, drawn, table?.concealed);

			// AND THIS IS WHERE BLOOD IS SHED, on the frame after the event that
			// changed somebody's hit points. It reads state.pawns rather than
			// the filtered list because it has to remember what happened on
			// every floor; only the viewed one is drawn on. See decals.ts.
			decals.watch(state.pawns, viewedID, cell, now);

			pawnPass.build(synthetic.length > 0 ? drawn.concat(synthetic) : drawn, tableGrid, sprites);

			pawnsDirty = false;
			lastCell = cell;
			lastEpoch = sprites.epoch();
		}

		// THE RULER'S CELLS ARE THE FLOOR AND GO UNDER THE PAWNS, so a pawn
		// standing on a highlighted square is not tinted by it.
		for (const ruler of table ? table.rulers(rulers) : []) {
			for (let i = 0; i < ruler.cells.length; i += 2) {
				const [cx, cy] = cellCentre(tableGrid, ruler.cells[i], ruler.cells[i + 1]);
				floorMarks.cell(cx - cell / 2, cy - cell / 2, cell, ruler.color, 0.16);
			}
		}

		// THE BLOOD IS THE FLOOR ITSELF and goes under everything standing on
		// it -- under the pawns, and under the ruler's highlighted cells as
		// well, because a measurement being taken right now has to stay readable
		// over scenery that has been there for ten minutes.
		decals.build(viewedID, now, sprites, decalPass);
		decalPass.draw(camera, sprites.texture(), canvas.width, canvas.height, dpr);

		// THE DRAWING IS ON THE FLOOR AND GOES UNDER THE CREATURES, which is
		// where a mark somebody made on the map belongs: a circle round three
		// goblins has the goblins standing IN it rather than behind it. It is
		// above the blood for the same reason the ruler's cells are -- a line
		// drawn deliberately outranks a stain that arrived on its own.
		//
		// AND IT IS ABOVE THE FOG FOR NEITHER ROLE. The GM's tint goes over it
		// two lines below, so a note left in an unrevealed room reads as hidden
		// on the GM's screen too; the player's cover goes over everything, so it
		// is not on their screen at all.
		strokes.sync(state.strokes, viewedID);
		strokes.live(state.strokes, viewedID, table?.inHand() ?? null);
		strokes.draw(camera, canvas.width, canvas.height, dpr);

		// THE GM'S FOG IS A TINT AND IT GOES UNDER THE CREATURES. What it marks
		// is "the party cannot see this", which is a fact about the floor rather
		// than about what is standing on it -- so it is drawn on the floor, at
		// half alpha, and the goblin the GM is walking through it stays solid
		// and legible on top. A cover over the pawns would hide the half of the
		// table the GM is actually working in.
		drawFog("gm");

		pulse.slow = slowBeat(now);
		pulse.heart = fastBeat(now);

		floorMarks.draw(camera, canvas.width, canvas.height, dpr);

		// WHOEVER IS ACTING WEARS A RING, AND IT GOES UNDER THEM. It is the mark
		// the turn-order strip puts on the acting line, drawn round the creature
		// that line stands for: the same component turning at the same six
		// seconds a revolution, in the variant a floor wants rather than the one
		// a card wants. See aura-pass.ts.
		//
		// UNDER, because that is where the DOM version is: the padding round the
		// content, with the content on top of it. A glow over a token would be a
		// wash of gold across the one face at the table that has to stay legible.
		//
		// A GROUPED LINE MARKS ALL OF THEM. Nine goblins acting on one count are
		// nine ids on one entry and nine rings on the floor, which is the same
		// answer the camera frames itself against -- see actingPawnIds.
		const acting = actingPawnIds(state.initiative);
		let glowing = false;

		auras.begin();
		if (acting.length > 0) {
			for (const pawn of state.pawns) {
				if (pawn.layerId !== viewedID || !acting.includes(pawn.id)) {
					continue;
				}

				const [halfW, halfH] = pawnExtents(pawn, cell);

				// AN AMBUSHER'S AURA IS AS FAINT AS THE AMBUSHER, which is the
				// condition rings' rule and matters more here: this is the GM's copy
				// alone, and a ring at full strength is the loudest thing on the
				// table -- it would announce a creature the players cannot see to
				// anybody looking over the GM's shoulder.
				auras.add(
					pawn.x, pawn.y, halfW, halfH,
					auraColor(healthOf(pawn)), pawn.visible ? 1 : HIDDEN_ALPHA,
					pawn.kind === "object" ? AURA_RECT : AURA_DISC,
					pawn.kind === "object" ? pawn.rotation : 0,
				);

				glowing = true;
			}
		}

		auras.draw(camera, canvas.width, canvas.height, dpr, auraTurn(now));

		pawnPass.draw(camera, canvas.width, canvas.height, dpr, pulse);

		// THE RINGS ARE BUILT PER FRAME AND THE PAWNS ARE NOT, which is not an
		// inconsistency: there are a handful of rings and hundreds of pawns,
		// and what drives the rings -- a selection, a drag ghost, a heartbeat --
		// changes at the rate a hand moves rather than at the rate the table
		// does.
		rings.begin();
		for (const pawn of state.pawns) {
			if (pawn.layerId !== viewedID || pawn.kind === "object" || pawn.conditions.length === 0) {
				continue;
			}

			const [halfW] = pawnExtents(pawn, cell);
			const shown = Math.min(pawn.conditions.length, CONDITION_RINGS_MAX);

			// A HIDDEN PAWN'S RINGS ARE AS FAINT AS THE PAWN IS. This is the GM's
			// copy alone -- nobody else is sent one -- and rings at full strength
			// round a half-drawn ambusher are louder than the creature they
			// belong to.
			const visible = pawn.visible ? 1 : HIDDEN_ALPHA;

			for (let i = 0; i < shown; i++) {
				const colour = CONDITION_COLORS[pawn.conditions[i].color] ?? CONDITION_COLORS.white;
				const radius = ringRadius(halfW, i, worldPerDevicePixel);

				rings.add(pawn.x, pawn.y, radius, radius, colour, visible, RING_WIDTH, RING_ELLIPSE);
			}
		}
		// The selection's rings and the marquee, which are the viewer's own and
		// belong over the pawn rather than round it.
		for (const outline of table ? table.outlines(outlines) : []) {
			rings.add(
				outline.x, outline.y, outline.halfW, outline.halfH,
				outline.color, outline.alpha, outline.thickness,
				outline.rect ? RING_RECT : RING_ELLIPSE, outline.rotation,
			);
		}

		rings.draw(camera, canvas.width, canvas.height, dpr);

		// THE GHOSTS GO OVER EVERYTHING THEY ARE A PROPOSAL ABOUT. Half alpha,
		// so the committed pawn underneath stays legible -- a preview that hid
		// what it was replacing would be worse than no preview.
		if (table) {
			ghostPass.build(table.ghosts(ghosts), tableGrid, sprites, GHOST_ALPHA);
			ghostPass.draw(camera, canvas.width, canvas.height, dpr);
		}

		// THE HANDLES GO OVER THE GHOST, which is why they are a second batch
		// through the same pass rather than more instances in the first. What a
		// hand is dragging is drawn at half alpha and the control doing the
		// dragging has to stay solid on top of it.
		//
		// A BOX FOR A CORNER AND A CIRCLE FOR THE ROTATION, both a fixed size on
		// screen, both turned with the token so the set reads as one frame round
		// it rather than as marks scattered near it.
		if (table) {
			const half = HANDLE_HALF * worldPerCssPixel;

			rings.begin();
			for (const handle of table.handles(handles)) {
				rings.add(
					handle.x, handle.y, half, half,
					SELECT_COLOR, 1, HANDLE_WIDTH,
					handle.turns ? RING_ELLIPSE : RING_RECT, handle.rotation,
				);
			}
			rings.draw(camera, canvas.width, canvas.height, dpr);
		}

		// THE PINGS GO OVER THE CREATURES AND UNDER THE PLAYER'S COVER.
		//
		// OVER THE CREATURES BECAUSE THE THING BEING POINTED AT IS USUALLY A
		// CREATURE. The drawing goes UNDER them -- a circle round three goblins
		// has the goblins standing in it -- but a ring closing on a square
		// somebody is standing on has to be visible over the token standing
		// there, or it is pointing at nothing.
		//
		// AND UNDER THE COVER BECAUSE A PING IS SOMEBODY ELSE'S AND CAN POINT
		// INTO FOG. The marks and the labels below this go OVER the cover, and
		// the note there says why: a measurement is the viewer's own mark on
		// their own screen, so reading it over the fog is how somebody measures
		// the distance to a door they have not opened. A ping is the opposite --
		// it arrives from another person and lands wherever they pressed -- so a
		// GM pinging into an unrevealed room must be pointing at nothing as far
		// as the players are concerned. Same rule as a stroke, same reason.
		//
		// AN EMPTY BATCH COSTS THE COUNT CHECK IN draw() AND NOTHING ELSE, which
		// is what makes a table nobody is pointing at free.
		rings.begin();
		pings.build(viewedID, now, cell, pingRings);
		rings.draw(camera, canvas.width, canvas.height, dpr);

		// AND THE PLAYER'S FOG IS A COVER AND IT GOES OVER EVERYTHING IT HIDES:
		// the map, the grid, the blood, the pawns, the rings round them and the
		// ghosts of somebody else's drag. At full alpha in the table's own
		// colour, so a covered floor is the exact shade of the empty desk and
		// there is no edge of the map anywhere on their screen.
		//
		// IT IS BEFORE THE RULER AND NOT AFTER IT. A measurement is the
		// viewer's own mark on their own screen and reading it over the fog is
		// how somebody measures the distance to a door they have not opened.
		drawFog("player");

		// The fog polygon a GM is clicking out goes here for the same reason,
		// and through the same pass: it is a mark on the table rather than a
		// thing on it.
		for (const segment of table ? table.marks(segments) : []) {
			overMarks.line(segment.x0, segment.y0, segment.x1, segment.y1, segment.width, segment.color, segment.alpha);
		}

		// THE DISTANCE ACROSS EVERY SHAPE, over the pawns standing in it. A
		// circle round three goblins is drawn UNDER them -- it is a mark on the
		// floor -- but the number that says how wide it is has to be read, and a
		// token sitting on top of it would be a template nobody can size.
		for (const label of table ? table.labels(labels) : []) {
			overMarks.label(label.text, label.x, label.y, label.color, label.alpha);
		}

		// And the ruler's line and its distance, last, because they are read
		// against everything else.
		for (const ruler of rulers) {
			overMarks.line(ruler.x0, ruler.y0, ruler.x1, ruler.y1, 2, ruler.color, 0.9);
			overMarks.label(ruler.label, (ruler.x0 + ruler.x1) / 2, (ruler.y0 + ruler.y1) / 2, ruler.color, 1);
		}

		overMarks.draw(camera, canvas.width, canvas.height, dpr);

		// sprites.end() FIRST AND ALWAYS, because it is what drains the loader
		// as well as what answers the question -- and || would skip it.
		//
		// pawnPass.beating() and glowing are the two answers with no end in
		// sight. The first is a creature on the viewed floor at a quarter of its
		// hit points or less, still alive; the second is somebody's turn. Both
		// are deliberate and both are bounded by the thing they are about: while
		// a heartbeat is running it is the most important thing on the table,
		// and a turn lasts as long as a turn lasts. Everything else here is
		// finite by construction.
		// pings.settling SWEEPS AS WELL AS ANSWERS and so is read into a local
		// rather than left in the chain below, where a short circuit would skip
		// it. build only ever sees the floor being LOOKED at, so without this a
		// ping left on a floor the GM walked away from would sit in the map
		// until the tab closed.
		const pointing = pings.settling(now);

		return sprites.end() || pawnPass.beating() || decals.settling(now) || pointing || glowing;
	}

	// drawFog puts the cover down. It is called at TWO points in drawPawns and
	// draws at exactly one of them: the parameter is whose cover this position
	// is for, and a call for the other role returns without touching the canvas.
	// The two positions are far apart in the draw order and each is explained
	// where it sits.
	//
	// A FLOOR WITH ITS FOG OFF DRAWS NOTHING AT ALL, which is what makes this
	// feature cost a room that is not using it exactly one branch per frame.
	function drawFog(whose: Role): void {
		if (whose !== role || !layers.viewed()?.fogEnabled) {
			return;
		}

		fog.draw(camera, canvas.width, canvas.height, dpr, clear, role === "gm" ? GM_FOG_ALPHA : PLAYER_FOG_ALPHA);
	}

	// settleMap notices the viewed map changing and frames it when framing is
	// what somebody would want.
	//
	// A FLOOR OF THE SAME SIZE KEEPS THE CAMERA. Floors of a building are
	// aligned -- that is the whole reason the grid is room-wide -- so a GM
	// stepping from the ground floor to the cellar is looking at the same
	// corner of the same building, and moving the camera would throw away the
	// one thing they were doing. Only a map that is a different SHAPE, or the
	// first map of the session, is worth a fit.
	function settleMap(map: MapRef | null): void {
		const key = map ? `${map.assetId}:${map.gen}` : "";
		if (key === lastMap) {
			return;
		}

		lastMap = key;
		if (!map) {
			return;
		}

		if (lastWidth !== map.width || lastHeight !== map.height) {
			fit(camera, viewport, map.width, map.height);

			// AND A FIT ENDS ANY TRAVEL, because a map of a different shape is a
			// different place: a camera still easing towards a box measured on
			// the map that has just been replaced would pull straight back off
			// the frame this just chose.
			travel = null;
		}

		lastWidth = map.width;
		lastHeight = map.height;
	}

	// advanceTravel eases the camera one frame along its way to whatever it was
	// pointed at, and clears the trip on the frame it arrives.
	//
	// THE ZOOM IS INTERPOLATED GEOMETRICALLY AND THE POSITION IS NOT, because a
	// zoom is a ratio and a position is a distance. Halfway between 0.25 and 4
	// on a straight line is 2.125, which is most of the way to the far end;
	// halfway between them as a ratio is 1, which is what "half zoomed" looks
	// like. Interpolating it linearly reads as the table rushing away at the
	// start of the move and crawling at the end of it.
	//
	// THE EASE IS A SMOOTHSTEP, so the camera starts and finishes at rest. That
	// is the difference between a table that glides to the next creature and
	// one that is yanked there and stopped dead.
	function advanceTravel(now: number): void {
		if (!travel) {
			return;
		}

		const t = Math.min(1, (now - travel.from) / FOCUS_MS);
		const eased = t * t * (3 - 2 * t);

		camera.x = travel.x + (target.x - travel.x) * eased;
		camera.y = travel.y + (target.y - travel.y) * eased;
		camera.zoom = travel.zoom * Math.pow(target.zoom / travel.zoom, eased);

		if (t >= 1) {
			travel = null;
		}
	}

	// advanceSweep drives the camera through the benchmark and answers whether
	// it is still running.
	//
	// THE SWEEP IS SCRIPTED AND NOT RANDOM, so two runs on two machines are
	// comparable and a change that made the renderer slower shows up as a
	// number rather than as a feeling. It pans the width of the map at three
	// zooms -- which crosses at least one level boundary and pulls tiles the
	// whole way -- and then zooms in and out at the centre, which is the motion
	// that thrashes the cache hardest.
	function advanceSweep(now: number, map: MapRef | null): boolean {
		if (!sweep) {
			return false;
		}

		const elapsed = now - sweep.from;
		if (elapsed >= BENCHMARK_MS || !map) {
			const timings = frames.timings();
			const report = sweep.report;
			const tilesBefore = sweep.tiles;
			sweep = null;

			report({
				average: timings.average,
				p95: timings.p95,
				frames: timings.samples,
				tiles: tiles.fetched() - tilesBefore,
			});

			return false;
		}

		const t = elapsed / BENCHMARK_MS;

		if (t < 0.75) {
			// Three passes across the map, each at a different zoom.
			const pass = Math.floor(t / 0.25);
			const within = (t % 0.25) / 0.25;

			camera.zoom = [fitZoom(map), 1, 2][pass] ?? 1;
			camera.x = map.width * within;
			camera.y = map.height / 2;
		} else {
			// A zoom in and back out at the centre, which is what thrashes the
			// tile cache hardest: every level of the pyramid in one motion.
			const within = (t - 0.75) / 0.25;
			const swing = 1 - Math.abs(within * 2 - 1);

			camera.x = map.width / 2;
			camera.y = map.height / 2;
			camera.zoom = fitZoom(map) * Math.pow(4 / fitZoom(map), swing);
		}

		clampToMap(camera, viewport, map.width, map.height);

		return true;
	}

	function fitZoom(map: MapRef): number {
		return Math.min(viewport.width / map.width, viewport.height / map.height) * 0.9;
	}

	function onViewCommand(e: CustomEvent<{ action?: string }>): void {
		const map = layers.viewed()?.map ?? null;

		switch (e.detail?.action) {
			case "zoom-in":
				zoomAt(camera, viewport, viewport.width / 2, viewport.height / 2, VIEW_ZOOM_STEP);
				break;
			case "zoom-out":
				zoomAt(camera, viewport, viewport.width / 2, viewport.height / 2, 1 / VIEW_ZOOM_STEP);
				break;
			case "zoom-1":
				zoomTo(camera, viewport, 1);
				break;
			case "zoom-2":
				zoomTo(camera, viewport, 2);
				break;
			case "fit":
				if (map) {
					fit(camera, viewport, map.width, map.height);
				}
				break;
			default:
				return;
		}

		if (map) {
			clampToMap(camera, viewport, map.width, map.height);
		}

		frames.invalidate();
	}

	// THE TABLETOP MENU'S CLEAR BLOOD, AND IT ANSWERS TO NOBODY BUT THE PERSON
	// WHO PRESSED IT. It comes over the same window-event gap the camera items
	// do -- see onViewCommand above and public/js/room.js at the other end --
	// because a menu item in one bundle and a renderer in another cannot import
	// each other. It carries nothing, because there is only one thing it means.
	//
	// EVERY FLOOR AND NOT THE VIEWED ONE. What somebody reaching for this wants
	// is a clean table, and a version of it that left last week's cellar red
	// would have to be pressed once per floor by somebody who cannot see the
	// floors they are pressing it for.
	//
	// NOTHING IS SENT AND NOTHING IS ASKED. Blood is not room state: it was
	// drawn here out of hit points this browser watched change, so there is no
	// command, no confirmation and no effect on anybody else's table. See wipe
	// in decals.ts, and the note beside it about why what a pawn was last seen
	// at is kept.
	function onBloodCommand(): void {
		decals.wipe();
		frames.invalidate();
	}

	// THE CLEAR COLOUR IS THE PAGE'S OWN, read from the element the canvas
	// covers, so the table and the chrome around it are the same shade and a
	// theme change moves both. It is resolved through a one pixel 2D canvas
	// rather than parsed, because a computed background-color is whatever
	// syntax the browser chose to serialise it as -- rgb(), colour with a
	// space, or the oklch() the themes are actually written in -- and canvas
	// resolves every one of them to the sRGB bytes that are wanted here.
	function readClearColor(): void {
		const probe = document.createElement("canvas").getContext("2d", { willReadFrequently: true });
		if (!probe) {
			return;
		}

		probe.fillStyle = "#000000";
		probe.fillStyle = window.getComputedStyle(mount).backgroundColor;
		probe.fillRect(0, 0, 1, 1);

		const pixel = probe.getImageData(0, 0, 1, 1).data;
		clear.r = pixel[0] / 255;
		clear.g = pixel[1] / 255;
		clear.b = pixel[2] / 255;

		frames.invalidate();
	}

	return {
		invalidate: frames.invalidate,
		view: layers,

		onSettled(fn) {
			settled.push(fn);
		},

		onFrame(fn) {
			framed = fn;
		},

		benchmark(report) {
			if (sweep) {
				return;
			}

			frames.resetTimings();
			sweep = { from: performance.now(), report, tiles: tiles.fetched() };
			frames.invalidate();
		},

		pawnsChanged() {
			pawnsDirty = true;
		},

		focus(rect) {
			focusTarget(camera, viewport, rect, target);

			// THE DESTINATION IS CLAMPED AND THE JOURNEY IS NOT. Both ends of
			// it are inside the bound -- the camera is already there, and this
			// is what puts the target there -- so nothing in between strays far
			// enough from the map to see, and clamping every frame against a
			// bound that moves with the zoom would bend the path.
			const map = layers.viewed()?.map ?? null;
			if (map) {
				clampToMap(target, viewport, map.width, map.height);
			}

			travel = { x: camera.x, y: camera.y, zoom: camera.zoom, from: performance.now() };
			frames.invalidate();
		},

		pinged(layerID, x, y, by) {
			// performance.now() RATHER THAN THE EVENT'S OWN CLOCK, because
			// there isn't one: a Pinged carries no timestamp, and one from the
			// server would be on a different clock from the frame loop's
			// anyway. What this measures is "when it got here", which for a
			// gesture that lasts a second is the only reading that matters.
			pings.add(layerID, x, y, actorColor(by), performance.now());
			frames.invalidate();
		},

		bloodCleared(layerID) {
			decals.clear(layerID);
			frames.invalidate();
		},

		bloodResync() {
			decals.resync();
		},

		showBlood(on) {
			decals.show(on);
			frames.invalidate();
		},

		toScreen: (x, y, out) => worldToScreen(camera, viewport, x, y, out),
		mapPerPixel: () => 1 / Math.max(camera.zoom, 1e-4),

		stress(count) {
			const map = layers.viewed()?.map ?? null;
			const cell = Math.max(1, state.table.grid.cellSize);

			synthetic = count > 0
				? stressPawns(count, drawn, cell, map ? map.width / 2 : 0, map ? map.height / 2 : 0)
				: [];

			pawnsDirty = true;
			frames.invalidate();

			return synthetic.length;
		},

		stop() {
			window.removeEventListener(THEME_CHANGE, readClearColor);
			window.removeEventListener(ROOM_VIEW, onViewCommand as EventListener);
			window.removeEventListener(ROOM_BLOOD, onBloodCommand);
			canvas.removeEventListener("webglcontextlost", onContextLost);
			canvas.removeEventListener("webglcontextrestored", onContextRestored);
			input.stop();
			frames.stop();
			teardown();
		},
	};
}
