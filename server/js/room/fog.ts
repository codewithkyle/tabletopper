



























import type { FogMode, FogShape, Grid, Pawn, Role, ShapeKind, State } from "./protocol.ts";
import type { Outgoing } from "./socket.ts";
import type { Modifiers } from "./render/input.ts";
import type { Outline, Segment } from "./pawns.ts";
import type { Point } from "./render/camera.ts";
import { snapAxis } from "./render/path.ts";
import { typing } from "./keys.ts";





const REVEAL_COLOR: readonly [number, number, number] = [1.0, 0.82, 0.35];
const HIDE_COLOR: readonly [number, number, number] = [0.55, 0.83, 0.99];


const PREVIEW_WIDTH = 2;
const PREVIEW_ALPHA = 0.95;






const VERTEX_FOOTPRINT = 2;




const EARS_MAX = 100_000;













export function snapCorner(grid: Grid, x: number, y: number, alt: boolean): [number, number] {
	if (alt) {
		return [Math.round(x), Math.round(y)];
	}

	return [
		Math.round(snapAxis(grid.cellSize, grid.offsetX, VERTEX_FOOTPRINT, grid.snap, x)),
		Math.round(snapAxis(grid.cellSize, grid.offsetY, VERTEX_FOOTPRINT, grid.snap, y)),
	];
}





export function rectTriangles(points: readonly number[], out: number[]): number[] {
	out.length = 0;
	if (points.length < 4) {
		return out;
	}

	const x0 = Math.min(points[0], points[2]);
	const y0 = Math.min(points[1], points[3]);
	const x1 = Math.max(points[0], points[2]);
	const y1 = Math.max(points[1], points[3]);

	out.push(x0, y0, x1, y0, x1, y1);
	out.push(x0, y0, x1, y1, x0, y1);

	return out;
}







export function triangulate(ring: readonly number[], out: number[]): number[] {
	out.length = 0;

	const n = ring.length >> 1;
	if (n < 3) {
		return out;
	}

	
	
	const left: number[] = [];
	for (let i = 0; i < n; i++) {
		left.push(i);
	}

	const clockwise = signedArea(ring) < 0;

	let guard = 0;
	let at = 0;
	while (left.length > 3 && guard++ < EARS_MAX) {
		const count = left.length;
		const prev = left[(at + count - 1) % count];
		const here = left[at % count];
		const next = left[(at + 1) % count];

		if (isEar(ring, left, prev, here, next, clockwise)) {
			out.push(ring[prev * 2], ring[prev * 2 + 1]);
			out.push(ring[here * 2], ring[here * 2 + 1]);
			out.push(ring[next * 2], ring[next * 2 + 1]);
			left.splice(at % count, 1);
			at = 0;

			continue;
		}

		at++;

		
		
		
		
		if (at > count) {
			break;
		}
	}

	for (let i = 1; i + 1 < left.length; i++) {
		out.push(ring[left[0] * 2], ring[left[0] * 2 + 1]);
		out.push(ring[left[i] * 2], ring[left[i] * 2 + 1]);
		out.push(ring[left[i + 1] * 2], ring[left[i + 1] * 2 + 1]);
	}

	return out;
}



function signedArea(ring: readonly number[]): number {
	let sum = 0;
	for (let i = 0, n = ring.length >> 1; i < n; i++) {
		const j = (i + 1) % n;
		sum += ring[i * 2] * ring[j * 2 + 1] - ring[j * 2] * ring[i * 2 + 1];
	}

	return sum;
}



function isEar(
	ring: readonly number[], left: readonly number[],
	prev: number, here: number, next: number, clockwise: boolean,
): boolean {
	const ax = ring[prev * 2], ay = ring[prev * 2 + 1];
	const bx = ring[here * 2], by = ring[here * 2 + 1];
	const cx = ring[next * 2], cy = ring[next * 2 + 1];

	const turn = cross(ax, ay, bx, by, cx, cy);
	if (clockwise ? turn >= 0 : turn <= 0) {
		return false;
	}

	for (const i of left) {
		if (i === prev || i === here || i === next) {
			continue;
		}
		if (inTriangle(ring[i * 2], ring[i * 2 + 1], ax, ay, bx, by, cx, cy)) {
			return false;
		}
	}

	return true;
}

function cross(ax: number, ay: number, bx: number, by: number, cx: number, cy: number): number {
	return (bx - ax) * (cy - ay) - (by - ay) * (cx - ax);
}

function inTriangle(
	px: number, py: number,
	ax: number, ay: number, bx: number, by: number, cx: number, cy: number,
): boolean {
	const d1 = cross(ax, ay, bx, by, px, py);
	const d2 = cross(bx, by, cx, cy, px, py);
	const d3 = cross(cx, cy, ax, ay, px, py);

	return !((d1 < 0 || d2 < 0 || d3 < 0) && (d1 > 0 || d2 > 0 || d3 > 0));
}














const bounds = new WeakMap<FogShape, [number, number, number, number]>();

function boxOf(shape: FogShape): [number, number, number, number] {
	const found = bounds.get(shape);
	if (found) {
		return found;
	}

	let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
	for (let i = 0; i + 1 < shape.points.length; i += 2) {
		const x = shape.points[i];
		const y = shape.points[i + 1];
		if (x < minX) minX = x;
		if (x > maxX) maxX = x;
		if (y < minY) minY = y;
		if (y > maxY) maxY = y;
	}

	const box: [number, number, number, number] = [minX, minY, maxX, maxY];
	bounds.set(shape, box);

	return box;
}





export function insideShape(shape: FogShape, x: number, y: number): boolean {
	const p = shape.points;
	const [minX, minY, maxX, maxY] = boxOf(shape);

	
	if (x < minX || x > maxX || y < minY || y > maxY) {
		return false;
	}
	if (shape.kind === "rect") {
		return p.length >= 4;
	}

	let inside = false;
	for (let i = 0, n = p.length >> 1, j = n - 1; i < n; j = i++) {
		const xi = p[i * 2], yi = p[i * 2 + 1];
		const xj = p[j * 2], yj = p[j * 2 + 1];

		if ((yi > y) !== (yj > y) && x < ((xj - xi) * (y - yi)) / (yj - yi) + xi) {
			inside = !inside;
		}
	}

	return inside;
}





export function coveredBy(
	shapes: readonly FogShape[], layerID: string, prefill: boolean, x: number, y: number,
): boolean {
	
	
	
	
	
	for (let i = shapes.length - 1; i >= 0; i--) {
		const shape = shapes[i];
		if (shape.layerId !== layerID) {
			continue;
		}
		if (insideShape(shape, x, y)) {
			return shape.mode === "hide";
		}
	}

	return prefill;
}













export function maskRect(
	map: { width: number; height: number } | null,
	shapes: readonly FogShape[], layerID: string, cell: number,
	out: MaskRect,
): MaskRect | null {
	let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;

	if (map && map.width > 0 && map.height > 0) {
		minX = 0;
		minY = 0;
		maxX = map.width;
		maxY = map.height;
	}

	for (const shape of shapes) {
		if (shape.layerId !== layerID) {
			continue;
		}
		for (let i = 0; i + 1 < shape.points.length; i += 2) {
			const x = shape.points[i];
			const y = shape.points[i + 1];
			if (x < minX) minX = x;
			if (x > maxX) maxX = x;
			if (y < minY) minY = y;
			if (y > maxY) maxY = y;
		}
	}

	if (!(maxX > minX) || !(maxY > minY)) {
		return null;
	}

	
	
	
	const pad = Math.max(cell, 1);
	out.x = minX - pad;
	out.y = minY - pad;
	out.width = maxX - minX + pad * 2;
	out.height = maxY - minY + pad * 2;

	return out;
}





export interface MaskRect {
	x: number;
	y: number;
	width: number;
	height: number;
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

	function layer(): { fogEnabled: boolean; fogPrefill: boolean } | null {
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

	function previewColor(): readonly [number, number, number] {
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
			
			
			
			
			if (deps.role === "gm" || (pawn.ownerId !== null && pawn.ownerId === deps.user)) {
				return false;
			}

			return covered(pawn.x, pawn.y);
		},
	};
}
