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

import type { Camera, Viewport } from "./camera.ts";
import type { MapRef, State } from "../protocol.ts";
import type { Drawn } from "./pawn-pass.ts";
import type { LayerView } from "./layers.ts";
import { clampToMap, fit, newCamera, zoomAt, zoomTo } from "./camera.ts";
import { createContext } from "./gl.ts";
import { createGlyphAtlas } from "./glyphs.ts";
import { createGridPass } from "./grid-pass.ts";
import { createPathPass } from "./path-pass.ts";
import { createPawnPass } from "./pawn-pass.ts";
import { createRingPass, RING_ELLIPSE } from "./ring-pass.ts";
import { createSpriteCache, CONDITION_COLORS } from "./sprites.ts";
import { createTilePass } from "./tile-pass.ts";
import { newLayerView } from "./layers.ts";
import { startFrames } from "./frame.ts";
import { pawnExtents } from "./pawn-pass.ts";
import { CONDITION_RINGS_MAX, RING_WIDTH, ringRadius, visiblePawns } from "./scene.ts";
import { stressPawns } from "./stress.ts";
import { apply, wireInput } from "./input.ts";

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

	// stress adds synthetic pawns beside the real ones, for the benchmark.
	// Nothing about them is sent anywhere; see stress.ts.
	stress(count: number): number;

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
export function mountRenderer(mount: HTMLElement, state: State): Renderer | null {
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
	const rings = createRingPass(gl);
	const atlas = createGlyphAtlas(gl);

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
	let lastViewed = "";
	let lastFollowing = true;

	// The pawn instance buffer is rebuilt on a change rather than per frame,
	// and these are every way it can change: the reducer said so, the floor
	// moved, the cell size moved, or a picture landed and with it the aspect
	// ratio a quad is fitted to.
	const drawn: Drawn[] = [];
	let synthetic: Drawn[] = [];
	let pawnsDirty = true;
	let lastCell = 0;
	let lastEpoch = -1;

	const input = wireInput(canvas, () => frames.invalidate());

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

		const loading = drawPawns(viewedID);

		// Five reasons to draw again, and the loop stops when none of them
		// holds: a button or a finger is down, the crossfade is partway
		// through, tiles or pictures are queued for upload, or the benchmark is
		// driving.
		return input.dragging() || layers.fading() || uploading || loading || sweeping;
	}

	// drawPawns is the table's contents: the highlighted cells under them, the
	// pawns themselves, and the rings round them.
	//
	// THE ORDER IS THE ORDER OF THE TABLE, as it is for the tiles and the grid.
	// A highlight is the floor and goes under the pawn standing on it; a
	// condition ring is on the pawn and goes over it; the ruler's line and its
	// distance are read against everything else and go last.
	function drawPawns(viewedID: string): boolean {
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

			pawnPass.build(synthetic.length > 0 ? drawn.concat(synthetic) : drawn, tableGrid, sprites);

			pawnsDirty = false;
			lastCell = cell;
			lastEpoch = sprites.epoch();
		}

		floorMarks.draw(camera, canvas.width, canvas.height, dpr);
		pawnPass.draw(camera, canvas.width, canvas.height, dpr);

		// THE RINGS ARE BUILT PER FRAME AND THE PAWNS ARE NOT, which is not an
		// inconsistency: there are a handful of rings and hundreds of pawns,
		// and what will drive the rings in the next phase -- a selection, a
		// drag ghost -- changes at the rate a hand moves rather than at the
		// rate the table does.
		rings.begin();
		for (const pawn of state.pawns) {
			if (pawn.layerId !== viewedID || pawn.kind === "object" || pawn.conditions.length === 0) {
				continue;
			}

			const [halfW] = pawnExtents(pawn, cell);
			const shown = Math.min(pawn.conditions.length, CONDITION_RINGS_MAX);

			for (let i = 0; i < shown; i++) {
				const colour = CONDITION_COLORS[pawn.conditions[i].color] ?? CONDITION_COLORS.white;
				const radius = ringRadius(halfW, i, worldPerDevicePixel);

				rings.add(pawn.x, pawn.y, radius, radius, colour, 1, RING_WIDTH, RING_ELLIPSE);
			}
		}
		rings.draw(camera, canvas.width, canvas.height, dpr);

		overMarks.draw(camera, canvas.width, canvas.height, dpr);

		return sprites.end();
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
			rings.dispose();
			floorMarks.dispose();
			overMarks.dispose();
			sprites.dispose();
			atlas?.dispose();
		},
	};
}
