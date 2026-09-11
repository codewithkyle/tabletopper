




















import type { Grid, Role, State, Stroke, StrokeKind } from "./protocol.ts";
import type { Outgoing } from "./socket.ts";
import type { Modifiers } from "./render/input.ts";
import type { Label, Outline, Segment } from "./pawns.ts";
import type { Point } from "./render/camera.ts";
import { distanceLabel, feetBetween } from "./render/path.ts";
import { parseColor } from "./render/grid-pass.ts";
import { typing } from "./keys.ts";
import { ulid } from "./ulid.ts";








const CHUNK_MS = 100;
const CHUNK_POINTS = 64;





export const DEFAULT_WIDTH = 4;





const ERASE_RADIUS = 6;





const PREVIEW_WIDTH = 2;




const ERASE_COLOR: readonly [number, number, number] = [0.98, 0.98, 0.99];
const ERASE_WIDTH = 1.5;









export type DrawMode = "pen" | "rect" | "circle" | "cone" | "erase";



const SHAPES: readonly DrawMode[] = ["rect", "circle", "cone"];


type Shaped = "rect" | "circle" | "cone";

export interface DrawOptions {
	mode: DrawMode;
	color: string;
	width: number;
}

export interface DrawDeps {
	state: State;
	role: Role;
	user: string;
	viewed: () => string;

	
	
	
	grid: () => Grid;
	send: (command: Outgoing) => void;
	invalidate: () => void;

	
	
	
	scale: () => number;

	
	
	
	drawing: () => boolean;

	options: () => DrawOptions;
}

export interface Draw {
	press(map: Point, mods: Modifiers): boolean;
	drag(map: Point, mods: Modifiers): void;
	release(map: Point, mods: Modifiers): void;

	
	
	secondary(): boolean;

	abandon(): boolean;

	
	key(e: KeyboardEvent): boolean;

	
	
	hover(map: Point | null): void;

	
	
	
	
	
	
	
	outline(): Outline | null;

	
	
	marks(out: Segment[]): Segment[];

	
	
	labels(out: Label[]): Label[];

	
	
	
	inHand(): Stroke | null;
}















export function strokeSegments(stroke: { kind: StrokeKind; points: readonly number[] }, out: number[]): number[] {
	const p = stroke.points;

	switch (stroke.kind) {
		case "free": {
			if (p.length < 2) {
				return out;
			}
			if (p.length === 2) {
				out.push(p[0], p[1], p[0], p[1]);

				return out;
			}

			for (let i = 0; i + 3 < p.length; i += 2) {
				out.push(p[i], p[i + 1], p[i + 2], p[i + 3]);
			}

			return out;
		}

		case "rect": {
			if (p.length < 4) {
				return out;
			}

			const x0 = Math.min(p[0], p[2]);
			const y0 = Math.min(p[1], p[3]);
			const x1 = Math.max(p[0], p[2]);
			const y1 = Math.max(p[1], p[3]);

			out.push(x0, y0, x1, y0);
			out.push(x1, y0, x1, y1);
			out.push(x1, y1, x0, y1);
			out.push(x0, y1, x0, y0);

			return out;
		}

		case "circle": {
			if (p.length < 4) {
				return out;
			}

			const r = Math.hypot(p[2] - p[0], p[3] - p[1]);
			if (r <= 0) {
				return out;
			}

			const n = circleSegments(r);
			let px = p[0] + r;
			let py = p[1];

			for (let i = 1; i <= n; i++) {
				const a = (i / n) * Math.PI * 2;
				const x = p[0] + Math.cos(a) * r;
				const y = p[1] + Math.sin(a) * r;
				out.push(px, py, x, y);
				px = x;
				py = y;
			}

			return out;
		}

		case "cone": {
			if (p.length < 4) {
				return out;
			}

			const corners = coneCorners(p[0], p[1], p[2], p[3], []);
			if (corners.length === 0) {
				return out;
			}

			out.push(corners[0], corners[1], corners[2], corners[3]);
			out.push(corners[2], corners[3], corners[4], corners[5]);
			out.push(corners[4], corners[5], corners[0], corners[1]);

			return out;
		}

		default:
			
			
			
			return out;
	}
}














export function coneCorners(
	ax: number, ay: number, bx: number, by: number, out: number[],
): number[] {
	out.length = 0;

	const dx = bx - ax;
	const dy = by - ay;
	const length = Math.hypot(dx, dy);
	if (length <= 0) {
		return out;
	}

	
	
	const half = length / 2;
	const nx = (-dy / length) * half;
	const ny = (dx / length) * half;

	out.push(ax, ay);
	out.push(bx + nx, by + ny);
	out.push(bx - nx, by - ny);

	return out;
}














export function circleSegments(radius: number): number {
	return Math.max(24, Math.min(512, Math.ceil((Math.PI * 2 * radius) / 4)));
}


















const bounds = new WeakMap<Stroke, [number, number, number, number]>();

function boxOf(stroke: Stroke): [number, number, number, number] {
	const found = bounds.get(stroke);
	if (found) {
		return found;
	}

	let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
	for (let i = 0; i + 1 < stroke.points.length; i += 2) {
		const x = stroke.points[i];
		const y = stroke.points[i + 1];
		if (x < minX) minX = x;
		if (x > maxX) maxX = x;
		if (y < minY) minY = y;
		if (y > maxY) maxY = y;
	}

	const box: [number, number, number, number] = [minX, minY, maxX, maxY];
	bounds.set(stroke, box);

	return box;
}




export function strokeHit(stroke: Stroke, x: number, y: number, radius: number, out: number[]): boolean {
	const reach = radius + Math.max(stroke.width, 1) / 2;

	const [minX, minY, maxX, maxY] = boxOf(stroke);
	if (x < minX - reach || x > maxX + reach || y < minY - reach || y > maxY + reach) {
		return false;
	}

	out.length = 0;
	strokeSegments(stroke, out);

	const limit = reach * reach;
	for (let i = 0; i + 3 < out.length; i += 4) {
		if (segmentDistanceSquared(x, y, out[i], out[i + 1], out[i + 2], out[i + 3]) <= limit) {
			return true;
		}
	}

	return false;
}




function segmentDistanceSquared(
	px: number, py: number, x0: number, y0: number, x1: number, y1: number,
): number {
	const dx = x1 - x0;
	const dy = y1 - y0;
	const length = dx * dx + dy * dy;

	
	
	let t = 0;
	if (length > 0) {
		t = Math.min(1, Math.max(0, ((px - x0) * dx + (py - y0) * dy) / length));
	}

	const nx = px - (x0 + t * dx);
	const ny = py - (y0 + t * dy);

	return nx * nx + ny * ny;
}




















export function measure(
	kind: StrokeKind, points: readonly number[], grid: Grid, color: string,
	add: (text: string, x: number, y: number, color: string) => void,
): void {
	if (points.length < 4) {
		return;
	}

	
	
	
	
	
	
	if (kind === "circle") {
		const dx = points[2] - points[0];
		const dy = points[3] - points[1];
		const r = Math.hypot(dx, dy);
		if (r <= 0) {
			return;
		}

		add(distanceLabel(feetBetween(dx, dy, grid)), points[0], points[1] - r, color);

		return;
	}

	if (kind === "cone") {
		const dx = points[2] - points[0];
		const dy = points[3] - points[1];
		if (dx === 0 && dy === 0) {
			return;
		}

		const corners = coneCorners(points[0], points[1], points[2], points[3], []);
		if (corners.length === 0) {
			return;
		}

		let minX = Infinity, maxX = -Infinity, minY = Infinity;
		for (let i = 0; i + 1 < corners.length; i += 2) {
			minX = Math.min(minX, corners[i]);
			maxX = Math.max(maxX, corners[i]);
			minY = Math.min(minY, corners[i + 1]);
		}

		add(distanceLabel(feetBetween(dx, dy, grid)), (minX + maxX) / 2, minY, color);

		return;
	}

	if (kind !== "rect") {
		return;
	}

	const x0 = Math.min(points[0], points[2]);
	const y0 = Math.min(points[1], points[3]);
	const x1 = Math.max(points[0], points[2]);
	const y1 = Math.max(points[1], points[3]);

	
	
	
	if (x1 > x0) {
		add(distanceLabel(feetBetween(x1 - x0, 0, grid)), (x0 + x1) / 2, y0, color);
	}
	if (y1 > y0) {
		add(distanceLabel(feetBetween(0, y1 - y0, grid)), x0, (y0 + y1) / 2, color);
	}
}

function blankLabel(): Label {
	return { text: "", x: 0, y: 0, color: [1, 1, 1], alpha: 1 };
}

export function createDraw(deps: DrawDeps): Draw {
	
	
	let local: Stroke | null = null;

	
	let pending: number[] = [];
	let sentAt = 0;

	
	
	
	let step = 1;

	
	
	
	
	let rubbing: Set<string> | null = null;

	
	
	let pointer: Point | null = null;

	
	const ring: Outline = {
		x: 0, y: 0, halfW: 0, halfH: 0,
		color: ERASE_COLOR, alpha: 0.9, thickness: ERASE_WIDTH,
		rect: false, rotation: 0,
	};

	
	const segments: number[] = [];

	
	
	
	let shaping: { kind: Shaped; x0: number; y0: number; x1: number; y1: number } | null = null;

	
	
	const corners: number[] = [];

	
	
	
	
	
	
	const previewColor: [number, number, number] = [1, 1, 1];
	const preview: Outline = {
		x: 0, y: 0, halfW: 0, halfH: 0,
		color: previewColor, alpha: 0.95, thickness: PREVIEW_WIDTH,
		rect: false, rotation: 0,
	};

	
	
	const tint = new Float32Array(4);

	
	
	function paintPreview(): void {
		parseColor(deps.options().color, tint);
		previewColor[0] = tint[0];
		previewColor[1] = tint[1];
		previewColor[2] = tint[2];
	}

	
	
	
	
	
	
	
	
	
	
	
	
	
	function erasable(stroke: Stroke): boolean {
		return stroke.done
			&& stroke.layerId === deps.viewed()
			&& (deps.role === "gm" || stroke.by === deps.user);
	}

	
	
	function radius(): number {
		return ERASE_RADIUS * deps.scale();
	}

	
	function rub(map: Point): void {
		if (!rubbing) {
			return;
		}

		const reach = radius();
		for (const stroke of deps.state.strokes) {
			if (rubbing.has(stroke.id) || !erasable(stroke)) {
				continue;
			}
			if (strokeHit(stroke, map.x, map.y, reach, segments)) {
				rubbing.add(stroke.id);
			}
		}
	}

	function flush(): void {
		if (!local || pending.length === 0) {
			return;
		}

		deps.send({ type: "stroke.extend", id: local.id, points: pending });
		pending = [];
		sentAt = performance.now();
	}

	
	
	
	
	function keep(x: number, y: number): boolean {
		if (!local) {
			return false;
		}

		const n = local.points.length;
		const dx = x - local.points[n - 2];
		const dy = y - local.points[n - 1];

		return dx * dx + dy * dy >= step * step;
	}

	function add(x: number, y: number): void {
		if (!local) {
			return;
		}

		local.points.push(x, y);
		pending.push(x, y);

		if (pending.length >= CHUNK_POINTS * 2 || performance.now() - sentAt >= CHUNK_MS) {
			flush();
		}

		deps.invalidate();
	}

	
	
	
	function finish(): void {
		if (!local) {
			return;
		}

		flush();
		deps.send({ type: "stroke.end", id: local.id });
		local = null;
		deps.invalidate();
	}

	
	
	
	
	
	
	
	
	
	
	
	
	function place(shape: { kind: Shaped; x0: number; y0: number; x1: number; y1: number }): void {
		const layer = deps.viewed();
		if (layer === "") {
			return;
		}

		if (shape.kind === "rect") {
			if (shape.x0 === shape.x1 || shape.y0 === shape.y1) {
				return;
			}
		} else if (shape.x0 === shape.x1 && shape.y0 === shape.y1) {
			return;
		}

		const { color, width } = deps.options();

		deps.send({
			type: "stroke.begin",
			id: ulid(),
			layer,
			kind: shape.kind,
			color,
			width,
			
			
			
			
			points: shape.kind === "rect"
				? [
					Math.min(shape.x0, shape.x1), Math.min(shape.y0, shape.y1),
					Math.max(shape.x0, shape.x1), Math.max(shape.y0, shape.y1),
				]
				: [shape.x0, shape.y0, shape.x1, shape.y1],
		});
	}

	function abandon(): boolean {
		
		
		
		
		if (rubbing) {
			rubbing = null;
			deps.invalidate();

			return true;
		}

		
		
		if (shaping) {
			shaping = null;
			deps.invalidate();

			return true;
		}

		if (!local) {
			return false;
		}

		const id = local.id;
		finish();
		deps.send({ type: "stroke.erase", ids: [id] });

		return true;
	}

	
	
	
	
	
	
	
	
	
	
	function undo(): boolean {
		const strokes = deps.state.strokes;

		for (let i = strokes.length - 1; i >= 0; i--) {
			const stroke = strokes[i];
			if (!stroke.done || stroke.layerId !== deps.viewed() || stroke.by !== deps.user) {
				continue;
			}

			deps.send({ type: "stroke.erase", ids: [stroke.id] });

			return true;
		}

		return false;
	}

	return {
		press(map, mods) {
			if (!deps.drawing()) {
				return false;
			}

			const layer = deps.viewed();
			if (layer === "") {
				return false;
			}

			
			
			
			
			
			
			finish();

			pointer = { x: map.x, y: map.y };

			
			
			
			const mode = deps.options().mode;

			if (mode === "erase") {
				rubbing = new Set();
				rub(map);
				deps.invalidate();

				return true;
			}

			
			
			
			
			
			if (SHAPES.includes(mode)) {
				shaping = {
					kind: mode as Shaped,
					x0: Math.round(map.x), y0: Math.round(map.y),
					x1: Math.round(map.x), y1: Math.round(map.y),
				};
				deps.invalidate();

				return true;
			}

			const { color, width } = deps.options();
			const x = Math.round(map.x);
			const y = Math.round(map.y);

			step = Math.max(1, deps.scale());

			
			
			
			
			
			
			local = {
				id: ulid(),
				by: "",
				layerId: layer,
				kind: "free",
				color,
				width,
				points: [x, y],
				done: false,
			};

			pending = [];
			sentAt = performance.now();

			deps.send({
				type: "stroke.begin",
				id: local.id,
				layer,
				kind: "free",
				color,
				width,
				points: [x, y],
			});

			deps.invalidate();

			return true;
		},

		drag(map, mods) {
			pointer = { x: map.x, y: map.y };

			if (rubbing) {
				rub(map);
				deps.invalidate();

				return;
			}

			if (shaping) {
				shaping.x1 = Math.round(map.x);
				shaping.y1 = Math.round(map.y);
				deps.invalidate();

				return;
			}

			if (!local) {
				return;
			}

			const x = Math.round(map.x);
			const y = Math.round(map.y);
			if (!keep(x, y)) {
				return;
			}

			add(x, y);
		},

		release(map, mods) {
			pointer = { x: map.x, y: map.y };

			if (rubbing) {
				rub(map);

				const ids = [...rubbing];
				rubbing = null;
				if (ids.length > 0) {
					deps.send({ type: "stroke.erase", ids });
				}
				deps.invalidate();

				return;
			}

			if (shaping) {
				shaping.x1 = Math.round(map.x);
				shaping.y1 = Math.round(map.y);

				const done = shaping;
				shaping = null;
				deps.invalidate();
				place(done);

				return;
			}

			if (!local) {
				return;
			}

			
			
			
			const x = Math.round(map.x);
			const y = Math.round(map.y);
			const n = local.points.length;
			if (local.points[n - 2] !== x || local.points[n - 1] !== y) {
				add(x, y);
			}

			finish();
		},

		secondary: abandon,

		
		
		
		
		
		
		
		
		abandon,

		
		
		
		key(e) {
			if (!deps.drawing() || typing(e.target)) {
				return false;
			}

			if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "z") {
				
				
				
				
				undo();

				return true;
			}

			return false;
		},

		hover(map) {
			pointer = map ? { x: map.x, y: map.y } : null;
		},

		
		
		
		
		
		
		outline() {
			if (!deps.drawing()) {
				return null;
			}

			if (shaping) {
				
				
				if (shaping.kind === "cone") {
					return null;
				}

				paintPreview();

				if (shaping.kind === "rect") {
					
					
					
					const halfW = Math.abs(shaping.x1 - shaping.x0) / 2;
					const halfH = Math.abs(shaping.y1 - shaping.y0) / 2;
					if (halfW <= 0 || halfH <= 0) {
						return null;
					}

					preview.x = (shaping.x0 + shaping.x1) / 2;
					preview.y = (shaping.y0 + shaping.y1) / 2;
					preview.halfW = halfW;
					preview.halfH = halfH;
					preview.rect = true;

					return preview;
				}

				const r = Math.hypot(shaping.x1 - shaping.x0, shaping.y1 - shaping.y0);
				if (r <= 0) {
					return null;
				}

				preview.x = shaping.x0;
				preview.y = shaping.y0;
				preview.halfW = r;
				preview.halfH = r;
				preview.rect = false;

				return preview;
			}

			if (deps.options().mode !== "erase" || !pointer) {
				return null;
			}

			const reach = radius();
			ring.x = pointer.x;
			ring.y = pointer.y;
			ring.halfW = reach;
			ring.halfH = reach;

			return ring;
		},

		
		
		
		
		marks(out) {
			if (!deps.drawing() || shaping?.kind !== "cone") {
				return out;
			}

			coneCorners(shaping.x0, shaping.y0, shaping.x1, shaping.y1, corners);
			if (corners.length === 0) {
				return out;
			}

			paintPreview();

			for (let i = 0; i < 3; i++) {
				const j = (i + 1) % 3;
				out.push({
					x0: corners[i * 2], y0: corners[i * 2 + 1],
					x1: corners[j * 2], y1: corners[j * 2 + 1],
					color: previewColor, alpha: 0.95, width: PREVIEW_WIDTH,
				});
			}

			return out;
		},

		
		
		
		
		
		
		
		
		
		
		
		
		
		labels(out) {
			let count = 0;

			if (shaping) {
				measure(
					shaping.kind,
					[shaping.x0, shaping.y0, shaping.x1, shaping.y1],
					deps.grid(), deps.options().color,
					(text, x, y, color) => {
						
						
						
						
						
						const slot = out[count] ?? (out[count] = blankLabel());
						parseColor(color, tint);

						slot.text = text;
						slot.x = x;
						slot.y = y;
						slot.color[0] = tint[0];
						slot.color[1] = tint[1];
						slot.color[2] = tint[2];
						slot.alpha = 1;
						count++;
					},
				);
			}

			out.length = count;

			return out;
		},

		inHand() {
			return local;
		},
	};
}
