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

import type { Camera, Point, Viewport } from "./camera.ts";
import type { MapRef, State } from "../protocol.ts";
import type { Drawn } from "./pawn-pass.ts";
import type { LayerView } from "./layers.ts";
import { clampToMap, fit, newCamera, zoomAt, zoomTo } from "./camera.ts";
import { createContext } from "./gl.ts";
import { createGlyphAtlas } from "./glyphs.ts";
import { createGridPass } from "./grid-pass.ts";
import { createDecalPass } from "./decal-pass.ts";
import { createPathPass } from "./path-pass.ts";
import { HIDDEN_ALPHA, createPawnPass } from "./pawn-pass.ts";
import type { PawnPulse } from "./pawn-pass.ts";
import { createRingPass, RING_ELLIPSE, RING_RECT } from "./ring-pass.ts";
import { createSpriteCache, CONDITION_COLORS } from "./sprites.ts";
import { createTilePass } from "./tile-pass.ts";
import { newDecals } from "./decals.ts";
import { newLayerView } from "./layers.ts";
import { startFrames } from "./frame.ts";
import { pawnExtents } from "./path.ts";
import { CONDITION_RINGS_MAX, RING_WIDTH, ringRadius, visiblePawns } from "./scene.ts";
import { cellCentre } from "./path.ts";
import { fastBeat, slowBeat } from "./wounds.ts";
import { stressPawns } from "./stress.ts";
import type { Outline, Ruler, Table } from "../pawns.ts";
import type { Handle } from "../handles.ts";
import { HANDLE_HALF } from "../handles.ts";
import { GHOST_ALPHA, SELECT_COLOR } from "../pawns.ts";
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
	// that makes the two differ lives in here and nowhere else.
	onSettled(fn: () => void): void;

	stop(): void;
}

// mountRenderer answers null when there is nothing to draw on, which is a
// browser without WebGL2 or a page that rendered no canvas. Neither is an
// error: the room's menus, windows and player list are ordinary HTML and go on
// working, and the caller has nothing to do about it.
export function mountRenderer(mount: HTMLElement, state: State, table?: Table): Renderer | null {
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
	const grid = createGridPass(gl);
	const tiles = createTilePass(gl, () => frames.invalidate());
	const sprites = createSpriteCache(gl, () => frames.invalidate());
	const pawnPass = createPawnPass(gl);

	// A SECOND PAWN PASS FOR THE GHOSTS, because they are the one thing on the
	// table that changes every frame. The committed pawns are rebuilt on a
	// change and the previews are rebuilt continuously; sharing one buffer
	// would mean rebuilding the whole table at the rate of a hand.
	const ghostPass = createPawnPass(gl);
	const rings = createRingPass(gl);
	const atlas = createGlyphAtlas(gl);

	// THE BLOOD IS A PASS AND A POOL. The pass draws tinted quads off the sprite
	// cache; the pool decides what is on the floor and how dry it is, and it is
	// the only thing here that keeps state the store does not. See decals.ts.
	const decalPass = createDecalPass(gl);
	const decals = newDecals();

	// TWO PATH PASSES, UNDER AND OVER. The highlighted cells are the floor and
	// go beneath the pawns standing on them; the line and the distance label
	// are the ruler and go on top. See path-pass.ts.
	const floorMarks = createPathPass(gl, atlas);
	const overMarks = createPathPass(gl, atlas);

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

	let settled: (() => void) | null = null;
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
	window.addEventListener("theme:change", readClearColor);
	window.addEventListener("room:view", onViewCommand as EventListener);

	frames.invalidate();

	function drawFrame(): boolean {
		const now = performance.now();

		layers.update(state.table, now);

		const viewedID = layers.viewed()?.id ?? "";
		const following = layers.following();
		if (viewedID !== lastViewed || following !== lastFollowing) {
			lastViewed = viewedID;
			lastFollowing = following;
			settled?.();
		}

		const painted = layers.draws();
		const map = layers.viewed()?.map ?? null;

		settleMap(map);

		const sweeping = advanceSweep(now, map);
		if (!sweeping && apply(input.pending, camera, viewport) && map) {
			clampToMap(camera, viewport, map.width, map.height);
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
		// driving. drawPawns folds in two more of its own -- blood that is
		// still drying, and a creature with a heartbeat -- and only the last of
		// those has no end in sight. See the note beside it.
		return input.dragging() || layers.fading() || uploading || loading || sweeping;
	}

	// drawPawns is the table's contents: the blood and the highlighted cells
	// under them, the pawns themselves, and the rings round them.
	//
	// THE ORDER IS THE ORDER OF THE TABLE, as it is for the tiles and the grid.
	// Blood is the floor itself and goes under everything; a highlight is on the
	// floor and goes under the pawn standing on it; a condition ring is on the
	// pawn and goes over it; the ruler's line and its distance are read against
	// everything else and go last.
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
			visiblePawns(state.pawns, viewedID, drawn);

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

		pulse.slow = slowBeat(now);
		pulse.heart = fastBeat(now);

		floorMarks.draw(camera, canvas.width, canvas.height, dpr);
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
		// pawnPass.beating() is the one answer here with no end in sight: a
		// creature on the viewed floor at a quarter of its hit points or less,
		// still alive. That is a wider window than it sounds -- it is most
		// monsters for most of a fight -- and it is deliberate: while it holds,
		// the heartbeat is the most important thing on the table. Everything
		// else is finite by construction.
		return sprites.end() || pawnPass.beating() || decals.settling(now);
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
		}

		lastWidth = map.width;
		lastHeight = map.height;
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
			settled = fn;
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

		bloodCleared(layerID) {
			decals.clear(layerID);
			frames.invalidate();
		},

		bloodResync() {
			decals.resync();
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
			window.removeEventListener("theme:change", readClearColor);
			window.removeEventListener("room:view", onViewCommand as EventListener);
			input.stop();
			frames.stop();
			tiles.dispose();
			grid.dispose();
			pawnPass.dispose();
			ghostPass.dispose();
			decalPass.dispose();
			rings.dispose();
			floorMarks.dispose();
			overMarks.dispose();
			sprites.dispose();
			atlas?.dispose();
		},
	};
}
