import type { Grid, Role, State, Stroke } from "./protocol.ts";
import type { Outgoing } from "./socket.ts";
import type { Modifiers } from "./render/input.ts";
import type { Label, Outline, Segment } from "./pawns.ts";
import type { Point, Rgb } from "./model/types.ts";
import { parseColor } from "./model/color.ts";
import { coneCorners, measure, strokeHit } from "./model/stroke.ts";
import { typing } from "./keys.ts";
import { ulid } from "./ulid.ts";
const CHUNK_MS = 100;
const CHUNK_POINTS = 64;
export const DEFAULT_WIDTH = 4;
const ERASE_RADIUS = 6;
const PREVIEW_WIDTH = 2;
const ERASE_COLOR: Rgb = [0.98, 0.98, 0.99];
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
