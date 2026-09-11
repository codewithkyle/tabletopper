import type { FogMode, Grid, Layer, Pawn, Role, ShapeKind, State } from "./protocol.ts";
import type { Outgoing } from "./socket.ts";
import type { Modifiers } from "./render/input.ts";
import type { Outline, Segment } from "./pawns.ts";
import type { Point, Rgb } from "./model/types.ts";
import { snapAxis } from "./model/grid.ts";
import { concealed as concealedBy, coveredBy } from "./model/polygon.ts";
import { typing } from "./keys.ts";
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
	role: Role;
	user: string;
	viewed: () => string;
	grid: () => Grid;
	send: (command: Outgoing) => void;
	invalidate: () => void;
	fogging: () => boolean;
	options: () => FogOptions;
}
export interface Fog {
	press(map: Point, mods: Modifiers): boolean;
	drag(map: Point, mods: Modifiers): void;
	release(map: Point, mods: Modifiers): void;
	secondary(): boolean;
	hover(map: Point | null): void;
	key(e: KeyboardEvent): boolean;
	abandon(): boolean;
	outline(): Outline | null;
	marks(out: Segment[]): Segment[];
	covered(x: number, y: number): boolean;
	concealed(pawn: Pawn): boolean;
}
type Gesture =
	| { kind: "rect"; x0: number; y0: number; x1: number; y1: number }
	| { kind: "poly"; points: number[] }
	| null;
export function createFog(deps: FogDeps): Fog {
	const state = deps.state;
	let gesture: Gesture = null;
	let pointer: Point | null = null;
	const box: Outline = {
		x: 0, y: 0, halfW: 0, halfH: 0,
		color: REVEAL_COLOR, alpha: PREVIEW_ALPHA, thickness: PREVIEW_WIDTH,
		rect: true, rotation: 0,
	};
	function layer(): Layer | null {
		const id = deps.viewed();
		for (const l of state.table.layers) {
			if (l.id === id) {
				return l;
			}
		}
		return null;
	}
	function corner(map: Point, mods: Modifiers): [number, number] {
		return snapCorner(deps.grid(), map.x, map.y, mods.alt);
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
	function covered(x: number, y: number): boolean {
		const l = layer();
		if (!l || !l.fogEnabled) {
			return false;
		}
		return coveredBy(state.fog, deps.viewed(), l.fogPrefill, x, y);
	}
	return {
		press(map, mods) {
			if (!deps.fogging()) {
				return false;
			}
			const [x, y] = corner(map, mods);
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
		drag(map, mods) {
			if (gesture?.kind !== "rect") {
				return;
			}
			const [x, y] = corner(map, mods);
			gesture.x1 = x;
			gesture.y1 = y;
			deps.invalidate();
		},
		release(map, mods) {
			if (gesture?.kind !== "rect") {
				return;
			}
			const [x, y] = corner(map, mods);
			const { x0, y0 } = gesture;
			gesture = null;
			deps.invalidate();
			if (x === x0 || y === y0) {
				return;
			}
			send("rect", [Math.min(x0, x), Math.min(y0, y), Math.max(x0, x), Math.max(y0, y)]);
		},
		secondary() {
			if (gesture?.kind === "poly") {
				return finishPolygon();
			}
			if (gesture?.kind === "rect") {
				gesture = null;
				deps.invalidate();
				return true;
			}
			return false;
		},
		hover(map) {
			pointer = map ? { x: map.x, y: map.y } : null;
		},
		key(e) {
			if (!deps.fogging() || typing(e.target)) {
				return false;
			}
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
		abandon() {
			if (!gesture) {
				return false;
			}
			gesture = null;
			deps.invalidate();
			return true;
		},
		outline() {
			if (gesture?.kind !== "rect") {
				return null;
			}
			const halfW = Math.abs(gesture.x1 - gesture.x0) / 2;
			const halfH = Math.abs(gesture.y1 - gesture.y0) / 2;
			if (halfW <= 0 || halfH <= 0) {
				return null;
			}
			box.x = (gesture.x0 + gesture.x1) / 2;
			box.y = (gesture.y0 + gesture.y1) / 2;
			box.halfW = halfW;
			box.halfH = halfH;
			box.color = previewColor();
			return box;
		},
		marks(out) {
			if (gesture?.kind !== "poly" || gesture.points.length < 2) {
				return out;
			}
			const points = gesture.points;
			const color = previewColor();
			for (let i = 0; i + 3 < points.length; i += 2) {
				out.push({
					x0: points[i], y0: points[i + 1],
					x1: points[i + 2], y1: points[i + 3],
					color, alpha: PREVIEW_ALPHA, width: PREVIEW_WIDTH,
				});
			}
			if (pointer) {
				const last = points.length - 2;
				out.push({
					x0: points[last], y0: points[last + 1],
					x1: pointer.x, y1: pointer.y,
					color, alpha: PREVIEW_ALPHA * 0.7, width: PREVIEW_WIDTH,
				});
				if (points.length >= 4) {
					out.push({
						x0: pointer.x, y0: pointer.y,
						x1: points[0], y1: points[1],
						color, alpha: PREVIEW_ALPHA * 0.4, width: PREVIEW_WIDTH,
					});
				}
			}
			return out;
		},
		covered,
		concealed(pawn) {
			return concealedBy(pawn, state.fog, layer(), deps.role, deps.user);
		},
	};
}
