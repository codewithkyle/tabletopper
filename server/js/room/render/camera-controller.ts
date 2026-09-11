import { ROOM_VIEW } from "../../../public/js/events.js";
import type { Camera, Viewport } from "./camera.ts";
import type { MapRef } from "../protocol.ts";
import type { Rect } from "../model/types.ts";
import { clampToMap, fit, fitZoom, focusTarget, newCamera, zoomAt, zoomTo } from "./camera.ts";
const VIEW_ZOOM_STEP = 1.5;
const FOCUS_MS = 320;
const BENCHMARK_MS = 10_000;
export interface Benchmark {
	average: number;
	p95: number;
	frames: number;
	tiles: number;
}
export interface Timings {
	average: number;
	p95: number;
	samples: number;
}
export interface CameraDeps {
	camera: Camera;
	viewport: Viewport;
	invalidate(): void;
	timings(): Timings;
	resetTimings(): void;
	fetched(): number;
}
export interface CameraController {
	settleMap(map: MapRef | null): void;
	sweeping(now: number, map: MapRef | null): boolean;
	advance(now: number): void;
	dropTravel(): void;
	travelling(): boolean;
	focus(rect: Rect, map: MapRef | null): void;
	benchmark(report: (result: Benchmark) => void): void;
	stop(): void;
}
export function newCameraController(deps: CameraDeps): CameraController {
	const camera = deps.camera;
	const viewport = deps.viewport;
	const target: Camera = newCamera();
	let travel: { x: number; y: number; zoom: number; from: number } | null = null;
	let sweep: { from: number; report: (result: Benchmark) => void; tiles: number } | null = null;
	let lastMap = "";
	let lastWidth = 0;
	let lastHeight = 0;
	function zoomOf(map: MapRef): number {
		return fitZoom(viewport, map.width, map.height);
	}
	function onViewCommand(e: Event): void {
		const detail = (e as CustomEvent<{ action?: string; map?: MapRef | null }>).detail;
		const map = viewedMap;
		switch (detail?.action) {
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
		deps.invalidate();
	}
	let viewedMap: MapRef | null = null;
	window.addEventListener(ROOM_VIEW, onViewCommand);
	return {
		settleMap(map) {
			viewedMap = map;
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
		},
		sweeping(now, map) {
			if (!sweep) {
				return false;
			}
			const elapsed = now - sweep.from;
			if (elapsed >= BENCHMARK_MS || !map) {
				const timings = deps.timings();
				const report = sweep.report;
				const before = sweep.tiles;
				sweep = null;
				report({
					average: timings.average,
					p95: timings.p95,
					frames: timings.samples,
					tiles: deps.fetched() - before,
				});
				return false;
			}
			const t = elapsed / BENCHMARK_MS;
			if (t < 0.75) {
				const pass = Math.floor(t / 0.25);
				const within = (t % 0.25) / 0.25;
				camera.zoom = [zoomOf(map), 1, 2][pass] ?? 1;
				camera.x = map.width * within;
				camera.y = map.height / 2;
			} else {
				const within = (t - 0.75) / 0.25;
				const swing = 1 - Math.abs(within * 2 - 1);
				camera.x = map.width / 2;
				camera.y = map.height / 2;
				camera.zoom = zoomOf(map) * Math.pow(4 / zoomOf(map), swing);
			}
			clampToMap(camera, viewport, map.width, map.height);
			return true;
		},
		advance(now) {
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
		},
		dropTravel() {
			travel = null;
		},
		travelling: () => travel !== null,
		focus(rect, map) {
			focusTarget(camera, viewport, rect, target);
			if (map) {
				clampToMap(target, viewport, map.width, map.height);
			}
			travel = { x: camera.x, y: camera.y, zoom: camera.zoom, from: performance.now() };
			deps.invalidate();
		},
		benchmark(report) {
			if (sweep) {
				return;
			}
			deps.resetTimings();
			sweep = { from: performance.now(), report, tiles: deps.fetched() };
			deps.invalidate();
		},
		stop() {
			window.removeEventListener(ROOM_VIEW, onViewCommand);
		},
	};
}
