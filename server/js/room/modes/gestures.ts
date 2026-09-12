import type { Board } from "./board.ts";
import type { Ghostable, Handle } from "../model/overlay.ts";
import type { Modifiers } from "../render/input.ts";
import type { Pawn } from "../protocol.ts";
import type { Placed } from "../model/shape.ts";
import type { Point, Rect } from "../model/types.ts";
import type { Selection } from "../selection.ts";
import { SPIN_STEP, resized, turned } from "../handles.ts";
import { cellAt } from "../model/grid.ts";
import { dragSet } from "../selection.ts";
import { snapTo, snapsToGrid } from "../model/shape.ts";
const DRAG_THRESHOLD = 4;
const DRAG_INTERVAL = 1000 / 20;
const DOUBLE_MS = 400;
export interface Pressing {
	kind: "press";
	anchor: string;
	screen: Point;
	grab: Point;
	mods: Modifiers;
}
export interface Dragging {
	kind: "drag";
	anchor: string;
	ids: string[];
	origins: Map<string, Point>;
	grab: Point;
	ghost: Point;
	sent: [number, number] | null;
	sentAt: number;
}
export interface Shaping {
	kind: "shape";
	id: string;
	handle: Handle;
	width: number;
	height: number;
	rotation: number;
	moved: boolean;
}
export interface Marqueeing {
	kind: "marquee";
	from: Point;
	to: Point;
	screen: Point;
	moved: boolean;
	anchor: string | null;
}
export type Gesture = Pressing | Dragging | Shaping | Marqueeing | null;
const box: Rect = { x1: 0, y1: 0, x2: 0, y2: 0 };
const snapped: Point = { x: 0, y: 0 };
export function marqueeRect(active: Marqueeing): Rect {
	box.x1 = active.from.x;
	box.y1 = active.from.y;
	box.x2 = active.to.x;
	box.y2 = active.to.y;
	return box;
}
export function past(from: Point, now: Point): boolean {
	const dpr = window.devicePixelRatio || 1;
	return Math.hypot(now.x - from.x, now.y - from.y) * dpr >= DRAG_THRESHOLD;
}
export function shapeOf(gesture: Gesture, pawn: Pawn, out: Placed): Placed {
	out.kind = pawn.kind;
	out.size = pawn.size;
	out.x = pawn.x;
	out.y = pawn.y;
	out.width = pawn.width;
	out.height = pawn.height;
	out.rotation = pawn.rotation;
	if (gesture?.kind === "shape" && gesture.id === pawn.id) {
		out.width = gesture.width;
		out.height = gesture.height;
		out.rotation = gesture.rotation;
	}
	return out;
}
export function proposedGhost(gesture: Shaping, pawn: Pawn, out: Ghostable): Ghostable {
	out.id = pawn.id;
	out.kind = pawn.kind;
	out.name = pawn.name;
	out.image = pawn.image;
	out.x = pawn.x;
	out.y = pawn.y;
	out.z = pawn.z;
	out.size = pawn.size;
	out.width = gesture.width;
	out.height = gesture.height;
	out.rotation = gesture.rotation;
	return out;
}
export function beginDrag(board: Board, selection: Selection, from: Pressing): Dragging | null {
	const anchor = board.pawn(from.anchor);
	if (!anchor) {
		return null;
	}
	const ids = dragSet(board.state.pawns, anchor, selection, {
		withRiders: !from.mods.alt,
		role: board.role,
		user: board.user,
		cellSize: board.grid().cellSize,
	});
	const origins = new Map<string, Point>();
	for (const id of ids) {
		const p = board.pawn(id);
		if (p) {
			origins.set(id, { x: p.x, y: p.y });
		}
	}
	return {
		kind: "drag",
		anchor: anchor.id,
		ids,
		origins,
		grab: from.grab,
		ghost: { x: anchor.x, y: anchor.y },
		sent: null,
		sentAt: 0,
	};
}
export function moveDrag(board: Board, active: Dragging, map: Point): void {
	const anchor = board.pawn(active.anchor);
	if (!anchor) {
		return;
	}
	const grid = board.grid();
	const point = snapTo(grid, anchor, map.x + active.grab.x, map.y + active.grab.y, snapped);
	active.ghost.x = point.x;
	active.ghost.y = point.y;
	const cell = cellAt(grid, point.x, point.y);
	const now = performance.now();
	const moved = !active.sent || cell[0] !== active.sent[0] || cell[1] !== active.sent[1];
	const due = snapsToGrid(anchor, grid) ? moved : now - active.sentAt >= DRAG_INTERVAL;
	if (!due) {
		return;
	}
	active.sent = cell;
	active.sentAt = now;
	board.send({
		type: "pawn.drag",
		anchor: active.anchor,
		x: point.x,
		y: point.y,
		others: followers(board, active),
	});
}
export function commitDrag(board: Board, active: Dragging, cancelled: boolean): void {
	const anchor = board.pawn(active.anchor);
	const origin = active.origins.get(active.anchor);
	if (!anchor || !origin) {
		return;
	}
	board.send({
		type: "pawn.move",
		anchor: active.anchor,
		x: cancelled ? origin.x : active.ghost.x,
		y: cancelled ? origin.y : active.ghost.y,
		others: followers(board, active),
	});
	board.announce();
}
export function moveShape(board: Board, active: Shaping, map: Point, mods: Modifiers): void {
	const p = board.pawn(active.id);
	if (!p) {
		return;
	}
	if (active.handle.turns) {
		active.rotation = turned(p, map.x, map.y, mods.shift ? SPIN_STEP : 0);
	} else {
		const [width, height] = resized(p, active.handle, map.x, map.y, board.grid().cellSize, mods.shift);
		active.width = width;
		active.height = height;
	}
	active.moved = true;
	board.announce();
}
export function commitShape(board: Board, active: Shaping, cancelled: boolean): void {
	const p = board.pawn(active.id);
	if (!p || cancelled || !active.moved) {
		board.announce();
		return;
	}
	if (active.handle.turns) {
		if (active.rotation !== p.rotation) {
			board.send({ type: "pawn.update", id: p.id, rotation: active.rotation });
		}
	} else if (active.width !== p.width || active.height !== p.height) {
		board.send({ type: "pawn.update", id: p.id, width: active.width, height: active.height });
	}
	board.announce();
}
export function clickedPawn(board: Board, active: Gesture): Pawn | null {
	if (active?.kind === "press") {
		return board.pawn(active.anchor);
	}
	if (active?.kind === "marquee" && !active.moved && active.anchor !== null) {
		return board.pawn(active.anchor);
	}
	return null;
}
export interface Clicks {
	count(id: string | null): boolean;
}
export function newClicks(): Clicks {
	let last: { id: string; at: number } | null = null;
	return {
		count(id) {
			const now = performance.now();
			if (id === null) {
				last = null;
				return false;
			}
			if (last !== null && last.id === id && now - last.at <= DOUBLE_MS) {
				last = null;
				return true;
			}
			last = { id, at: now };
			return false;
		},
	};
}
function followers(board: Board, active: Dragging): string[] {
	return active.ids.filter((id) => id !== active.anchor && board.pawn(id) !== null);
}
