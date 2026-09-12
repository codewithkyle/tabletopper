import type { Grid, Role, State, Stroke } from "../protocol.ts";
import type { Outgoing } from "../socket.ts";
import type { Point } from "../model/types.ts";
import type { Shaped, Sketch } from "./ink.ts";
import type { Tool } from "../render/input.ts";
import { newInk } from "./ink.ts";
import { strokeHit } from "../model/stroke.ts";
import { ulid } from "../ulid.ts";
const CHUNK_MS = 100;
const CHUNK_POINTS = 64;
export const DEFAULT_WIDTH = 4;
const ERASE_RADIUS = 6;
export type DrawMode = "pen" | "rect" | "circle" | "cone" | "erase";
const SHAPES: readonly DrawMode[] = ["rect", "circle", "cone"];
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
	options: () => DrawOptions;
}
export function createDraw(deps: DrawDeps): Tool {
	let local: Stroke | null = null;
	let pending: number[] = [];
	let sentAt = 0;
	let step = 1;
	let rubbing: Set<string> | null = null;
	let pointer: Point | null = null;
	let chosen = false;
	let shaping: Sketch | null = null;
	const marks: number[] = [];
	const ink = newInk({ grid: deps.grid, options: deps.options });
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
			if (strokeHit(stroke, map.x, map.y, reach, marks)) {
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
	function place(shape: Sketch): void {
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
		press(map) {
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
		drag(map) {
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
		release(map) {
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
		cancel: abandon,
		secondary: abandon,
		key(e) {
			if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "z") {
				undo();
				return true;
			}
			return false;
		},
		hover(map) {
			pointer = map ? { x: map.x, y: map.y } : null;
		},
		abandon,
		active: () => false,
		enter() {
			chosen = true;
		},
		leave() {
			chosen = false;
			pointer = null;
		},
		contribute(out) {
			ink.reset();
			if (!chosen) {
				return;
			}
			ink.show(out, shaping, pointer, radius());
			out.inHand = local;
		},
	};
}
