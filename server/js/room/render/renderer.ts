













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








const HANDLE_WIDTH = 2;




const VIEW_ZOOM_STEP = 1.5;









const FOCUS_MS = 320;




const BENCHMARK_MS = 10_000;

export interface Benchmark {
	average: number;
	p95: number;
	frames: number;
	tiles: number;
}

export interface Renderer {
	
	
	invalidate(): void;

	
	view: LayerView;

	
	
	benchmark(report: (result: Benchmark) => void): void;

	
	
	
	
	pawnsChanged(): void;

	
	
	
	
	
	
	focus(rect: Rect): void;

	
	
	
	
	
	
	bloodCleared(layerID: string): void;

	
	
	
	
	
	
	
	bloodResync(): void;

	
	
	
	
	
	pinged(layerID: string, x: number, y: number, by: string): void;

	
	
	
	
	
	
	
	
	showBlood(on: boolean): void;

	
	
	stress(count: number): number;

	
	
	
	toScreen(x: number, y: number, out: Point): Point;

	
	
	
	
	mapPerPixel(): number;

	
	
	
	
	
	
	
	
	
	onFrame(fn: () => void): void;

	
	
	
	
	
	onSettled(fn: () => void): void;

	stop(): void;
}





export function mountRenderer(mount: HTMLElement, state: State, role: Role, table?: Table): Renderer | null {
	const found = mount.querySelector("[data-tabletop-canvas]");
	if (!(found instanceof HTMLCanvasElement)) {
		return null;
	}

	
	
	
	const canvas: HTMLCanvasElement = found;

	const context = createContext(canvas);
	if (!context) {
		
		
		
		mount.querySelector("[data-tabletop-unsupported]")?.removeAttribute("hidden");
		canvas.hidden = true;

		return null;
	}

	const gl: WebGL2RenderingContext = context;

	
	
	
	
	
	
	
	let grid = createGridPass(gl);
	let tiles = createTilePass(gl, () => frames.invalidate());
	let sprites = createSpriteCache(gl, () => frames.invalidate());
	let pawnPass = createPawnPass(gl);

	
	
	
	
	let ghostPass = createPawnPass(gl);
	let rings = createRingPass(gl);

	
	
	
	
	
	let auras = createAuraPass(gl);
	let atlas = createGlyphAtlas(gl);

	
	
	
	
	
	let decalPass = createDecalPass(gl);
	const decals = newDecals();

	
	
	
	
	
	
	const pings = newPings();

	
	
	
	const pingRings: RingTarget = {
		ellipse(x, y, radius, color, alpha, thickness) {
			rings.add(x, y, radius, radius, color, alpha, thickness, RING_ELLIPSE);
		},
	};

	
	
	
	let floorMarks = createPathPass(gl, atlas);
	let overMarks = createPathPass(gl, atlas);

	
	
	
	
	
	let fog = createFogPass(gl);

	
	
	
	let strokes = createStrokePass(gl);

	
	
	
	
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

	
	
	
	let lost = false;
	const restoring = mount.querySelector("[data-tabletop-restoring]");

	
	
	
	function onContextLost(e: Event): void {
		e.preventDefault();
		lost = true;
		if (restoring instanceof HTMLElement) {
			restoring.hidden = false;
		}
	}

	
	
	
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

	
	
	
	let lastMap = "";
	let lastWidth = 0;
	let lastHeight = 0;

	let sweep: { from: number; report: (result: Benchmark) => void; tiles: number } | null = null;

	
	
	
	
	
	
	
	
	const target: Camera = newCamera();
	let travel: { x: number; y: number; zoom: number; from: number } | null = null;

	const settled: (() => void)[] = [];
	let framed: (() => void) | null = null;
	let lastViewed = "";
	let lastFollowing = true;

	
	
	
	
	
	
	
	
	const pulse: PawnPulse = { slow: 0, heart: 0 };

	
	
	
	
	
	const PLAYER_FOG_ALPHA = 1;
	const GM_FOG_ALPHA = 0.5;

	const drawn: Drawn[] = [];
	let synthetic: Drawn[] = [];
	let pawnsDirty = true;
	let lastCell = 0;
	let lastEpoch = -1;

	
	
	
	const input = wireInput(
		canvas,
		() => frames.invalidate(),
		(x, y, out) => screenToWorld(camera, viewport, x, y, out),
		table?.tool ?? null,
	);

	
	
	
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

		
		
		
		
		
		
		
		return input.dragging() || layers.fading() || uploading || loading || sweeping || travel !== null;
	}

	
	
	
	
	
	
	
	
	
	
	
	
	
	function drawPawns(viewedID: string, now: number): boolean {
		const tableGrid = state.table.grid;
		const cell = Math.max(1, tableGrid.cellSize);

		
		
		
		const rebuilding = pawnsDirty || cell !== lastCell || sprites.epoch() !== lastEpoch;

		sprites.begin(rebuilding);

		
		
		
		
		const worldPerDevicePixel = 1 / Math.max(camera.zoom * dpr, 1e-4);
		const worldPerCssPixel = 1 / Math.max(camera.zoom, 1e-4);

		floorMarks.begin(worldPerCssPixel);
		overMarks.begin(worldPerCssPixel);

		if (rebuilding) {
			
			
			
			
			
			
			visiblePawns(state.pawns, viewedID, drawn, table?.concealed);

			
			
			
			
			decals.watch(state.pawns, viewedID, cell, now);

			pawnPass.build(synthetic.length > 0 ? drawn.concat(synthetic) : drawn, tableGrid, sprites);

			pawnsDirty = false;
			lastCell = cell;
			lastEpoch = sprites.epoch();
		}

		
		
		for (const ruler of table ? table.rulers(rulers) : []) {
			for (let i = 0; i < ruler.cells.length; i += 2) {
				const [cx, cy] = cellCentre(tableGrid, ruler.cells[i], ruler.cells[i + 1]);
				floorMarks.cell(cx - cell / 2, cy - cell / 2, cell, ruler.color, 0.16);
			}
		}

		
		
		
		
		decals.build(viewedID, now, sprites, decalPass);
		decalPass.draw(camera, sprites.texture(), canvas.width, canvas.height, dpr);

		
		
		
		
		
		
		
		
		
		
		strokes.sync(state.strokes, viewedID);
		strokes.live(state.strokes, viewedID, table?.inHand() ?? null);
		strokes.draw(camera, canvas.width, canvas.height, dpr);

		
		
		
		
		
		
		drawFog("gm");

		pulse.slow = slowBeat(now);
		pulse.heart = fastBeat(now);

		floorMarks.draw(camera, canvas.width, canvas.height, dpr);

		
		
		
		
		
		
		
		
		
		
		
		
		
		const acting = actingPawnIds(state.initiative);
		let glowing = false;

		auras.begin();
		if (acting.length > 0) {
			for (const pawn of state.pawns) {
				if (pawn.layerId !== viewedID || !acting.includes(pawn.id)) {
					continue;
				}

				const [halfW, halfH] = pawnExtents(pawn, cell);

				
				
				
				
				
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

		
		
		
		
		
		rings.begin();
		for (const pawn of state.pawns) {
			if (pawn.layerId !== viewedID || pawn.kind === "object" || pawn.conditions.length === 0) {
				continue;
			}

			const [halfW] = pawnExtents(pawn, cell);
			const shown = Math.min(pawn.conditions.length, CONDITION_RINGS_MAX);

			
			
			
			
			const visible = pawn.visible ? 1 : HIDDEN_ALPHA;

			for (let i = 0; i < shown; i++) {
				const colour = CONDITION_COLORS[pawn.conditions[i].color] ?? CONDITION_COLORS.white;
				const radius = ringRadius(halfW, i, worldPerDevicePixel);

				rings.add(pawn.x, pawn.y, radius, radius, colour, visible, RING_WIDTH, RING_ELLIPSE);
			}
		}
		
		
		for (const outline of table ? table.outlines(outlines) : []) {
			rings.add(
				outline.x, outline.y, outline.halfW, outline.halfH,
				outline.color, outline.alpha, outline.thickness,
				outline.rect ? RING_RECT : RING_ELLIPSE, outline.rotation,
			);
		}

		rings.draw(camera, canvas.width, canvas.height, dpr);

		
		
		
		if (table) {
			ghostPass.build(table.ghosts(ghosts), tableGrid, sprites, GHOST_ALPHA);
			ghostPass.draw(camera, canvas.width, canvas.height, dpr);
		}

		
		
		
		
		
		
		
		
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

		
		
		
		
		
		
		
		
		
		
		
		
		
		
		
		
		
		
		
		rings.begin();
		pings.build(viewedID, now, cell, pingRings);
		rings.draw(camera, canvas.width, canvas.height, dpr);

		
		
		
		
		
		
		
		
		
		drawFog("player");

		
		
		
		for (const segment of table ? table.marks(segments) : []) {
			overMarks.line(segment.x0, segment.y0, segment.x1, segment.y1, segment.width, segment.color, segment.alpha);
		}

		
		
		
		
		for (const label of table ? table.labels(labels) : []) {
			overMarks.label(label.text, label.x, label.y, label.color, label.alpha);
		}

		
		
		for (const ruler of rulers) {
			overMarks.line(ruler.x0, ruler.y0, ruler.x1, ruler.y1, 2, ruler.color, 0.9);
			overMarks.label(ruler.label, (ruler.x0 + ruler.x1) / 2, (ruler.y0 + ruler.y1) / 2, ruler.color, 1);
		}

		overMarks.draw(camera, canvas.width, canvas.height, dpr);

		
		
		
		
		
		
		
		
		
		
		
		
		
		
		
		const pointing = pings.settling(now);

		return sprites.end() || pawnPass.beating() || decals.settling(now) || pointing || glowing;
	}

	
	
	
	
	
	
	
	
	function drawFog(whose: Role): void {
		if (whose !== role || !layers.viewed()?.fogEnabled) {
			return;
		}

		fog.draw(camera, canvas.width, canvas.height, dpr, clear, role === "gm" ? GM_FOG_ALPHA : PLAYER_FOG_ALPHA);
	}

	
	
	
	
	
	
	
	
	
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

			
			
			
			
			travel = null;
		}

		lastWidth = map.width;
		lastHeight = map.height;
	}

	
	
	
	
	
	
	
	
	
	
	
	
	
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
			
			const pass = Math.floor(t / 0.25);
			const within = (t % 0.25) / 0.25;

			camera.zoom = [fitZoom(map), 1, 2][pass] ?? 1;
			camera.x = map.width * within;
			camera.y = map.height / 2;
		} else {
			
			
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

	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	
	function onBloodCommand(): void {
		decals.wipe();
		frames.invalidate();
	}

	
	
	
	
	
	
	
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

			
			
			
			
			
			const map = layers.viewed()?.map ?? null;
			if (map) {
				clampToMap(target, viewport, map.width, map.height);
			}

			travel = { x: camera.x, y: camera.y, zoom: camera.zoom, from: performance.now() };
			frames.invalidate();
		},

		pinged(layerID, x, y, by) {
			
			
			
			
			
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
