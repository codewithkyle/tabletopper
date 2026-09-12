import type { FogMode, Grid, ShapeKind, State } from "../protocol.ts";
import type { Outgoing } from "../socket.ts";
import type { Outline, Segment } from "../model/overlay.ts";
import type { Point, Rgb } from "../model/types.ts";
import type { Tool } from "../render/input.ts";
import { blankOutline, blankSegment, pool } from "../model/overlay.ts";
import { snapAxis } from "../model/grid.ts";
const REVEAL_COLOR: Rgb = [1.0, 0.82, 0.35];
const HIDE_COLOR: Rgb = [0.55, 0.83, 0.99];
const PREVIEW_WIDTH = 2;
const PREVIEW_ALPHA = 0.95;
const VERTEX_FOOTPRINT = 2;
export function snapCorner(grid: Grid, x: number, y: number, alt: boolean): [number, number] {
	if (alt) {
		return [Math.round(x), Math.round(y)];
	}
	return [
		Math.round(snapAxis(grid.cellSize, grid.offsetX, VERTEX_FOOTPRINT, grid.snap, x)),
		Math.round(snapAxis(grid.cellSize, grid.offsetY, VERTEX_FOOTPRINT, grid.snap, y)),
	];
}
export interface FogOptions {
	shape: ShapeKind;
	mode: FogMode;
}
export interface FogDeps {
	state: State;
	viewed: () => string;
	grid: () => Grid;
	send: (command: Outgoing) => void;
	invalidate: () => void;
	options: () => FogOptions;
}
type Gesture =
	| { kind: "rect"; x0: number; y0: number; x1: number; y1: number }
	| { kind: "poly"; points: number[] }
	| null;
export function createFog(deps: FogDeps): Tool {
	const state = deps.state;
	let gesture: Gesture = null;
	let pointer: Point | null = null;
	const segments = pool(blankSegment);
	const box: Outline = blankOutline();
	box.alpha = PREVIEW_ALPHA;
	box.thickness = PREVIEW_WIDTH;
	box.rect = true;
	function corner(map: Point, alt: boolean): [number, number] {
		return snapCorner(deps.grid(), map.x, map.y, alt);
	}
	function send(kind: ShapeKind, points: number[]): void {
		const id = deps.viewed();
		if (id === "" || points.length < 4) {
			return;
		}
		deps.send({ type: "fog.add", layer: id, kind, mode: deps.options().mode, points });
	}
	function finishPolygon(): boolean {
		if (gesture?.kind !== "poly") {
			return false;
		}
		const points = gesture.points;
		gesture = null;
		deps.invalidate();
		if (points.length >= 6) {
			send("poly", points);
		}
		return true;
	}
	function undo(): void {
		const id = deps.viewed();
		for (let i = state.fog.length - 1; i >= 0; i--) {
			if (state.fog[i].layerId === id) {
				deps.send({ type: "fog.remove", id: state.fog[i].id });
				return;
			}
		}
	}
	function previewColor(): Rgb {
		return deps.options().mode === "hide" ? HIDE_COLOR : REVEAL_COLOR;
	}
	function line(out: Segment[], x0: number, y0: number, x1: number, y1: number, color: Rgb, alpha: number): void {
		const slot = segments.take();
		slot.x0 = x0;
		slot.y0 = y0;
		slot.x1 = x1;
		slot.y1 = y1;
		slot.color = color;
		slot.alpha = alpha;
		slot.width = PREVIEW_WIDTH;
		out.push(slot);
	}
	function abandon(): boolean {
		if (!gesture) {
			return false;
		}
		gesture = null;
		deps.invalidate();
		return true;
	}
	return {
		press(map, screen, mods) {
			const [x, y] = corner(map, mods.alt);
			if (deps.options().shape === "poly") {
				if (gesture?.kind !== "poly") {
					gesture = { kind: "poly", points: [] };
				}
				const points = gesture.points;
				const n = points.length;
				if (n < 2 || points[n - 2] !== x || points[n - 1] !== y) {
					points.push(x, y);
				}
			} else {
				gesture = { kind: "rect", x0: x, y0: y, x1: x, y1: y };
			}
			deps.invalidate();
			return true;
		},
		drag(map, screen, mods) {
			if (gesture?.kind !== "rect") {
				return;
			}
			const [x, y] = corner(map, mods.alt);
			gesture.x1 = x;
			gesture.y1 = y;
			deps.invalidate();
		},
		release(map, screen, mods) {
			if (gesture?.kind !== "rect") {
				return;
			}
			const [x, y] = corner(map, mods.alt);
			const { x0, y0 } = gesture;
			gesture = null;
			deps.invalidate();
			if (x === x0 || y === y0) {
				return;
			}
			send("rect", [Math.min(x0, x), Math.min(y0, y), Math.max(x0, x), Math.max(y0, y)]);
		},
		cancel: abandon,
		secondary() {
			if (gesture?.kind === "poly") {
				return finishPolygon();
			}
			return abandon();
		},
		hover(map) {
			pointer = map ? { x: map.x, y: map.y } : null;
		},
		key(e) {
			if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "z") {
				undo();
				return true;
			}
			if (e.ctrlKey || e.metaKey || e.altKey) {
				return false;
			}
			if (e.key === "Enter") {
				return finishPolygon();
			}
			if (e.key === "Backspace" && gesture?.kind === "poly") {
				gesture.points.length = Math.max(0, gesture.points.length - 2);
				if (gesture.points.length === 0) {
					gesture = null;
				}
				deps.invalidate();
				return true;
			}
			return false;
		},
		abandon,
		active: () => false,
		leave() {
			pointer = null;
		},
		contribute(out) {
			segments.reset();
			if (gesture?.kind === "rect") {
				const halfW = Math.abs(gesture.x1 - gesture.x0) / 2;
				const halfH = Math.abs(gesture.y1 - gesture.y0) / 2;
				if (halfW > 0 && halfH > 0) {
					box.x = (gesture.x0 + gesture.x1) / 2;
					box.y = (gesture.y0 + gesture.y1) / 2;
					box.halfW = halfW;
					box.halfH = halfH;
					box.color = previewColor();
					out.outlines.push(box);
				}
			}
			if (gesture?.kind !== "poly" || gesture.points.length < 2) {
				return;
			}
			const points = gesture.points;
			const color = previewColor();
			for (let i = 0; i + 3 < points.length; i += 2) {
				line(out.segments, points[i], points[i + 1], points[i + 2], points[i + 3], color, PREVIEW_ALPHA);
			}
			if (!pointer) {
				return;
			}
			const last = points.length - 2;
			line(out.segments, points[last], points[last + 1], pointer.x, pointer.y, color, PREVIEW_ALPHA * 0.7);
			if (points.length >= 4) {
				line(out.segments, pointer.x, pointer.y, points[0], points[1], color, PREVIEW_ALPHA * 0.4);
			}
		},
	};
}
