import type { Benchmark, CameraController } from "./camera-controller.ts";
import type { Camera, Viewport } from "./camera.ts";
import type { Event as RoomEvent, Role, State } from "../protocol.ts";
import type { FrameContext } from "./frame-context.ts";
import type { LayerView } from "./layers.ts";
import type { Point, Rect } from "../model/types.ts";
import type { RenderStats } from "./stats.ts";
import type { Revisions } from "../model/revisions.ts";
import type { StageList } from "./stages/list.ts";
import type { Tool } from "./input.ts";
import {
	AGAIN_DRAGGING,
	AGAIN_FADING,
	AGAIN_STAGES,
	AGAIN_SWEEPING,
	AGAIN_TRAVELLING,
	AGAIN_UPLOADS,
	bit,
} from "./reasons.ts";
import { apply, wireInput } from "./input.ts";
import { drawCounts, resetDrawCounts } from "../gl/counters.ts";
import { newGpuTimer } from "./gpu-timer.ts";
import { clampToMap, newCamera, screenToWorld, worldPerCssPixel, worldToScreen } from "./camera.ts";
import { createContext } from "../gl/context.ts";
import { newCameraController } from "./camera-controller.ts";
import { newFrame, sizeFrame } from "./frame-context.ts";
import { newLayerView } from "./layers.ts";
import { newOverlay } from "../model/overlay.ts";
import { newStageList } from "./stages/list.ts";
import { startFrames } from "./frame.ts";
import { watchContextLoss } from "./context-loss.ts";
import { watching } from "../model/revisions.ts";
import { watchTheme } from "./theme.ts";
export type { Benchmark } from "./camera-controller.ts";
export type { RenderStats, TextureStats } from "./stats.ts";
interface LoseContext {
	loseContext(): void;
	restoreContext(): void;
}
const RESTORE_MS = 750;
export interface Renderer {
	invalidate(): void;
	view: LayerView;
	benchmark(report: (result: Benchmark) => void): void;
	event(event: RoomEvent): void;
	focus(rect: Rect): void;
	showBlood(on: boolean): void;
	stress(count: number): number;
	toScreen(x: number, y: number, out: Point): Point;
	toWorld(x: number, y: number, out: Point): Point;
	mapPerPixel(): number;
	onFrame(fn: () => void): void;
	onSettled(fn: () => void): void;
	stats(): RenderStats;
	samples(out: Float64Array): number;
	timing(on: boolean): void;
	loseContext(): boolean;
	stop(): void;
}
export function mountRenderer(
	mount: HTMLElement, state: State, revisions: Revisions, role: Role, user: string, tool: Tool,
): Renderer | null {
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
	const layers = newLayerView(mount.dataset.role === "gm");
	const camera: Camera = newCamera();
	const viewport: Viewport = { width: 1, height: 1 };
	let dpr = window.devicePixelRatio || 1;
	const rebuild = watching(["pawns", "table", "fog"]);
	let builtFloor = "";
	let lastEpoch = -1;
	let lastViewed = "";
	let lastFollowing = true;
	let framed: (() => void) | null = null;
	let rebuilds = 0;
	let calls = 0;
	let instances = 0;
	let timed = false;
	const settled: (() => void)[] = [];
	const list: StageList = newStageList(gl, role, () => frames.invalidate());
	const overlay = newOverlay();
	const frame: FrameContext = newFrame(gl, camera, viewport, role, user, state, revisions, overlay, list.resources());
	const input = wireInput(
		canvas,
		() => frames.invalidate(),
		(x, y, out) => screenToWorld(camera, viewport, x, y, out),
		tool,
	);
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
	let gpu = newGpuTimer(gl);
	const theme = watchTheme(mount, () => frames.invalidate());
	frame.clear = theme.color;
	const loss = watchContextLoss(mount, canvas, () => {
		list.reset();
		list.timing(timed);
		frame.resources = list.resources();
		gpu.dispose();
		gpu = newGpuTimer(gl);
		rebuild.reset();
		lastEpoch = -1;
	});
	const controller: CameraController = newCameraController({
		camera,
		viewport,
		invalidate: () => frames.invalidate(),
		timings: () => frames.timings(),
		resetTimings: () => frames.resetTimings(),
		fetched: () => list.fetched(),
	});
	frames.invalidate();
	function drawFrame(): number {
		if (loss.lost()) {
			return 0;
		}
		const now = performance.now();
		layers.update(state.table, now);
		const viewed = layers.viewed();
		const viewedID = viewed?.id ?? "";
		const following = layers.following();
		if (viewedID !== lastViewed || following !== lastFollowing) {
			lastViewed = viewedID;
			lastFollowing = following;
			for (const fn of settled) {
				fn();
			}
		}
		const map = viewed?.map ?? null;
		controller.settleMap(map);
		const sweeping = controller.sweeping(now, map);
		if (!sweeping) {
			if (input.dragging()) {
				controller.dropTravel();
			}
			if (apply(input.pending, camera, viewport)) {
				if (map) {
					clampToMap(camera, viewport, map.width, map.height);
				}
			} else {
				controller.advance(now);
			}
		}
		const resources = list.resources();
		sizeFrame(frame, now, canvas.width, canvas.height, dpr);
		frame.viewed = viewed;
		frame.viewedID = viewedID;
		frame.cell = Math.max(1, state.table.grid.cellSize);
		frame.painted = layers.draws();
		frame.rebuild = rebuild.changed(revisions) || viewedID !== builtFloor || resources.sprites.epoch() !== lastEpoch;
		overlay.reset();
		tool.contribute(overlay);
		resources.begin(frame.rebuild);
		list.build(frame);
		if (frame.rebuild) {
			builtFloor = viewedID;
			lastEpoch = resources.sprites.epoch();
			rebuilds++;
		}
		gl.viewport(0, 0, canvas.width, canvas.height);
		gl.clearColor(frame.clear[0], frame.clear[1], frame.clear[2], 1);
		gl.clear(gl.COLOR_BUFFER_BIT);
		resetDrawCounts();
		if (timed) {
			gpu.begin();
		}
		list.draw(frame);
		if (timed) {
			gpu.end();
		}
		const counts = drawCounts();
		calls = counts.calls;
		instances = counts.instances;
		const again = list.settling(frame);
		const uploading = resources.end();
		framed?.();
		return bit(again, AGAIN_STAGES) |
			bit(uploading, AGAIN_UPLOADS) |
			bit(input.dragging(), AGAIN_DRAGGING) |
			bit(layers.fading(), AGAIN_FADING) |
			bit(sweeping, AGAIN_SWEEPING) |
			bit(controller.travelling(), AGAIN_TRAVELLING);
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
		benchmark: controller.benchmark,
		event(event) {
			list.event(event);
			frames.invalidate();
		},
		focus(rect) {
			controller.focus(rect, layers.viewed()?.map ?? null);
		},
		showBlood: (on) => list.showBlood(on),
		toScreen: (x, y, out) => worldToScreen(camera, viewport, x, y, out),
		toWorld: (x, y, out) => screenToWorld(camera, viewport, x, y, out),
		mapPerPixel: () => worldPerCssPixel(camera),
		stress(count) {
			const added = list.stress(count);
			rebuild.reset();
			frames.invalidate();
			return added;
		},
		samples: (out) => frames.history(out),
		timing(on) {
			timed = on;
			list.timing(on);
			frames.invalidate();
		},
		loseContext() {
			const ext = gl.getExtension("WEBGL_lose_context") as LoseContext | null;
			if (!ext) {
				return false;
			}
			ext.loseContext();
			window.setTimeout(() => ext.restoreContext(), RESTORE_MS);
			return true;
		},
		stats() {
			const timings = frames.timings();
			return {
				drawn: frames.drawn(),
				last: frames.last(),
				average: timings.average,
				p95: timings.p95,
				samples: timings.samples,
				again: frames.again(),
				rebuilds,
				calls,
				instances,
				painted: frame.painted.length,
				sprites: list.resources().sprites.stats(),
				tiles: list.textures(),
				zoom: camera.zoom,
				worldPerCssPixel: worldPerCssPixel(camera),
				x: camera.x,
				y: camera.y,
				level: list.level(),
				visible: list.visible(),
				width: viewport.width,
				height: viewport.height,
				deviceWidth: canvas.width,
				deviceHeight: canvas.height,
				dpr,
				losses: loss.losses(),
				gpu: gpu.elapsed(),
				gpuAvailable: gpu.available(),
				timing: timed,
				stages: list.timings(),
			};
		},
		stop() {
			gpu.dispose();
			theme.stop();
			controller.stop();
			loss.stop();
			input.stop();
			frames.stop();
			list.dispose();
		},
	};
}
