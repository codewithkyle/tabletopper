import type { Drawn } from "./render/pawn-pass.ts";
import type { Event, Grid, Pawn, PawnKind, Role, Size, State, Stroke } from "./protocol.ts";
import type { Modifiers, Tool } from "./render/input.ts";
import type { Outgoing } from "./socket.ts";
import type { Point, Rect, Rgb } from "./model/types.ts";
import { Selection, dragSet, marqueeSelect, mayMove } from "./selection.ts";
import type { Placed, Sized } from "./model/shape.ts";
import {
	cellAt,
	cellCentre,
	cellsMoved,
	distanceLabel,
	feetBetween,
	supercover,
} from "./model/grid.ts";
import {
	boundsOf,
	containsPoint,
	pawnExtents,
	snapPawn,
	snapsToGrid,
} from "./model/shape.ts";
import { SELECT_COLOR, SELF_COLOR, actorColor } from "./model/color.ts";
import type { Draw } from "./draw.ts";
import type { Fog } from "./fog.ts";
import type { Handle } from "./handles.ts";
import { SPIN_STEP, handleAt, handlesFor, resized, turned } from "./handles.ts";
import { typing } from "./keys.ts";
import { compareStack } from "./model/stack.ts";
const DRAG_THRESHOLD = 4;
const DOUBLE_MS = 400;
const DRAG_INTERVAL = 1000 / 20;
const PREVIEW_TIMEOUT = 3000;
export const GHOST_ALPHA = 0.5;
const MEASURE_POINT = 4;
const MEASURE_WIDTH = 2;
export interface Outline {
	x: number;
	y: number;
	halfW: number;
	halfH: number;
	color: Rgb;
	alpha: number;
	thickness: number;
	rect: boolean;
	rotation: number;
}
export interface Segment {
	x0: number;
	y0: number;
	x1: number;
	y1: number;
	color: Rgb;
	alpha: number;
	width: number;
}
export interface Label {
	text: string;
	x: number;
	y: number;
	color: [number, number, number];
	alpha: number;
}
export interface Ruler {
	cells: number[];
	x0: number;
	y0: number;
	x1: number;
	y1: number;
	label: string;
	color: Rgb;
}
export interface Armed {
	kind: PawnKind;
	id: string;
	name: string;
	image: string;
	visible: boolean;
	size: Size;
	width: number;
	height: number;
	hp: number;
	maxHp: number;
	ac: number;
}
type Ghostable = Pick<
	Drawn,
	"id" | "kind" | "name" | "image" | "x" | "y" | "z" | "size" | "width" | "height" | "rotation"
>;
export interface TableDeps {
	state: State;
	role: Role;
	user: string;
	viewed: () => string;
	send: (command: Outgoing) => void;
	invalidate: () => void;
	scale: () => number;
	details: (pawn: Pawn) => void;
	menu: (pawn: Pawn, screen: Point) => void;
	panning: () => boolean;
	measuring: () => boolean;
	pinging: () => boolean;
	fog: Fog | null;
	draw: Draw | null;
	remove: () => void;
}
export interface Table {
	tool: Tool;
	selection: Selection;
	focus(): Pawn | null;
	ghosts(out: Drawn[]): Drawn[];
	outlines(out: Outline[]): Outline[];
	rulers(out: Ruler[]): Ruler[];
	marks(out: Segment[]): Segment[];
	inHand(): Stroke | null;
	labels(out: Label[]): Label[];
	concealed(pawn: Pawn): boolean;
	handles(out: Handle[]): Handle[];
	bounds(): Rect | null;
	preview(event: Event): void;
	floorChanged(): void;
	arm(armed: Armed | null): void;
	isArmed(): boolean;
	onChange(fn: () => void): void;
	stop(): void;
}
interface Pressing {
	kind: "press";
	anchor: string;
	screen: Point;
	grab: Point;
	mods: Modifiers;
}
interface Dragging {
	kind: "drag";
	anchor: string;
	ids: string[];
	origins: Map<string, Point>;
	grab: Point;
	ghost: Point;
	sent: [number, number] | null;
	sentAt: number;
}
interface Shaping {
	kind: "shape";
	id: string;
	handle: Handle;
	width: number;
	height: number;
	rotation: number;
	moved: boolean;
}
interface Marqueeing {
	kind: "marquee";
	from: Point;
	to: Point;
	screen: Point;
	moved: boolean;
	anchor: string | null;
}
type Gesture = Pressing | Dragging | Shaping | Marqueeing | null;
interface Measured {
	from: Point;
	to: Point;
}
interface Preview {
	positions: { id: string; x: number; y: number }[];
	color: Rgb;
	at: number;
}
export function createTable(deps: TableDeps): Table {
	const { state, role, user } = deps;
	const selection = new Selection();
	const previews = new Map<string, Preview>();
	let gesture: Gesture = null;
	let hovered: string | null = null;
	let armed: Armed | null = null;
	let pointer: Point | null = null;
	let measured: Measured | null = null;
	let fogging = false;
	let inking = false;
	let lastClick: { id: string; at: number } | null = null;
	let changed: (() => void) | null = null;
	const snapped: Point = { x: 0, y: 0 };
	const handleList: Handle[] = [];
	const shape: Placed = {
		kind: "object", size: "medium", width: 0, height: 0, rotation: 0, x: 0, y: 0,
	};
	const proposal: Ghostable = {
		id: "", kind: "object", name: "", image: "",
		x: 0, y: 0, z: 0, size: "medium", width: 0, height: 0, rotation: 0,
	};
	function grid(): Grid {
		return state.table.grid;
	}
	function pawn(id: string): Pawn | null {
		return state.pawns.find((p) => p.id === id) ?? null;
	}
	function announce(): void {
		changed?.();
		deps.invalidate();
	}
	function measurement(): Measured | null {
		if (measured && !deps.measuring()) {
			measured = null;
		}
		return measured;
	}
	function aim(map: Point): void {
		const line = measurement();
		if (!line) {
			return;
		}
		line.to.x = map.x;
		line.to.y = map.y;
		deps.invalidate();
	}
	function onFloor(p: Pawn): boolean {
		return p.layerId === deps.viewed();
	}
	function shaped(p: Pawn): Placed {
		shape.kind = p.kind;
		shape.size = p.size;
		shape.x = p.x;
		shape.y = p.y;
		shape.width = p.width;
		shape.height = p.height;
		shape.rotation = p.rotation;
		if (gesture?.kind === "shape" && gesture.id === p.id) {
			shape.width = gesture.width;
			shape.height = gesture.height;
			shape.rotation = gesture.rotation;
		}
		return shape;
	}
	function selectedToken(): Pawn | null {
		const one = selection.only();
		const p = one ? pawn(one) : null;
		if (!p || p.kind !== "object" || !onFloor(p) || !mayMove(p, role, user)) {
			return null;
		}
		return p;
	}
	function described(): Pawn | null {
		const p = hovered ? pawn(hovered) : null;
		return p && p.kind !== "object" ? p : null;
	}
	function activeHandles(out: Handle[]): Handle[] {
		const p = selectedToken();
		if (!p) {
			out.length = 0;
			return out;
		}
		return handlesFor(shaped(p), grid().cellSize, deps.scale(), out);
	}
	function snapFor(p: Sized, x: number, y: number): Point {
		const [sx, sy] = snapPawn(grid(), p, Math.round(x), Math.round(y));
		snapped.x = sx;
		snapped.y = sy;
		return snapped;
	}
	function beginDrag(from: Pressing): void {
		const anchor = pawn(from.anchor);
		if (!anchor) {
			gesture = null;
			return;
		}
		const ids = dragSet(state.pawns, anchor, selection, {
			withRiders: !from.mods.alt,
			role,
			user,
			cellSize: grid().cellSize,
		});
		const origins = new Map<string, Point>();
		for (const id of ids) {
			const p = pawn(id);
			if (p) {
				origins.set(id, { x: p.x, y: p.y });
			}
		}
		gesture = {
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
	function moveDrag(active: Dragging, map: Point): void {
		const anchor = pawn(active.anchor);
		if (!anchor) {
			return;
		}
		const point = snapFor(anchor, map.x + active.grab.x, map.y + active.grab.y);
		active.ghost.x = point.x;
		active.ghost.y = point.y;
		const cell = cellAt(grid(), point.x, point.y);
		const now = performance.now();
		const moved = !active.sent || cell[0] !== active.sent[0] || cell[1] !== active.sent[1];
		const due = snapsToGrid(anchor, grid()) ? moved : now - active.sentAt >= DRAG_INTERVAL;
		if (!due) {
			return;
		}
		active.sent = cell;
		active.sentAt = now;
		deps.send({
			type: "pawn.drag",
			anchor: active.anchor,
			x: point.x,
			y: point.y,
			others: followers(active),
		});
	}
	function commit(active: Dragging, cancelled: boolean): void {
		const anchor = pawn(active.anchor);
		const origin = active.origins.get(active.anchor);
		gesture = null;
		if (!anchor || !origin) {
			return;
		}
		const x = cancelled ? origin.x : active.ghost.x;
		const y = cancelled ? origin.y : active.ghost.y;
		deps.send({
			type: "pawn.move",
			anchor: active.anchor,
			x,
			y,
			others: followers(active),
		});
		announce();
	}
	function followers(active: Dragging): string[] {
		return active.ids.filter((id) => id !== active.anchor && pawn(id) !== null);
	}
	function moveShape(active: Shaping, map: Point, mods: Modifiers): void {
		const p = pawn(active.id);
		if (!p) {
			return;
		}
		if (active.handle.turns) {
			active.rotation = turned(p, map.x, map.y, mods.shift ? SPIN_STEP : 0);
		} else {
			const [width, height] = resized(p, active.handle, map.x, map.y, grid().cellSize, mods.shift);
			active.width = width;
			active.height = height;
		}
		active.moved = true;
		announce();
	}
	function commitShape(active: Shaping, cancelled: boolean): void {
		const p = pawn(active.id);
		gesture = null;
		if (!p || cancelled || !active.moved) {
			announce();
			return;
		}
		if (active.handle.turns) {
			if (active.rotation !== p.rotation) {
				deps.send({ type: "pawn.update", id: p.id, rotation: active.rotation });
			}
		} else if (active.width !== p.width || active.height !== p.height) {
			deps.send({ type: "pawn.update", id: p.id, width: active.width, height: active.height });
		}
		announce();
	}
	function place(map: Point): void {
		if (!armed) {
			return;
		}
		const point = snapFor(armedShape(armed, grid().cellSize), map.x, map.y);
		const npc = armed.kind === "npc";
		deps.send({
			type: "pawn.spawn",
			kind: armed.kind,
			layer: deps.viewed(),
			x: point.x,
			y: point.y,
			visible: armed.visible,
			size: armed.kind === "object" ? undefined : armed.size,
			monsterId: armed.kind === "monster" ? armed.id : undefined,
			characterId: armed.kind === "player" ? armed.id : undefined,
			assetId: npc || armed.kind === "object" ? armed.id || undefined : undefined,
			name: npc || armed.kind === "object" ? armed.name : undefined,
			hp: npc ? armed.hp : undefined,
			maxHp: npc ? armed.maxHp : undefined,
			ac: npc ? armed.ac : undefined,
		});
	}
	function concealed(pawn: Pawn): boolean {
		return deps.fog?.concealed(pawn) ?? false;
	}
	function abandon(): boolean {
		if (armed) {
			arm(null);
			return true;
		}
		if (deps.fog?.abandon()) {
			fogging = false;
			return true;
		}
		if (deps.draw?.abandon()) {
			inking = false;
			return true;
		}
		if (measurement()) {
			measured = null;
			announce();
			return true;
		}
		switch (gesture?.kind) {
			case "drag":
				commit(gesture, true);
				return true;
			case "shape":
				commitShape(gesture, true);
				return true;
			case "marquee":
				gesture = null;
				announce();
				return true;
			default:
				return false;
		}
	}
	function countClick(id: string | null): boolean {
		const now = performance.now();
		if (id === null) {
			lastClick = null;
			return false;
		}
		if (lastClick !== null && lastClick.id === id && now - lastClick.at <= DOUBLE_MS) {
			lastClick = null;
			return true;
		}
		lastClick = { id, at: now };
		return false;
	}
	function clickedPawn(active: Gesture): Pawn | null {
		if (active?.kind === "press") {
			return pawn(active.anchor);
		}
		if (active?.kind === "marquee" && !active.moved && active.anchor !== null) {
			return pawn(active.anchor);
		}
		return null;
	}
	function onKeyDown(e: KeyboardEvent): void {
		if (typing(e.target)) {
			return;
		}
		if (deps.fog?.key(e)) {
			return;
		}
		if (deps.draw?.key(e)) {
			return;
		}
		if (e.key === "Escape") {
			abandon();
			return;
		}
		if (e.key === "Delete" && selection.size > 0) {
			deps.remove();
		}
	}
	function arm(next: Armed | null): void {
		armed = next;
		announce();
	}
	document.addEventListener("keydown", onKeyDown);
	const tool: Tool = {
		press(map, screen, mods) {
			pointer = { x: map.x, y: map.y };
			if (deps.panning()) {
				gesture = null;
				return false;
			}
			if (armed) {
				place(map);
				return true;
			}
			if (deps.fog?.press(map, mods)) {
				fogging = true;
				gesture = null;
				return true;
			}
			if (deps.draw?.press(map, mods)) {
				inking = true;
				gesture = null;
				return true;
			}
			if (deps.pinging()) {
				gesture = null;
				deps.send({
					type: "ping",
					layer: deps.viewed(),
					x: Math.round(map.x),
					y: Math.round(map.y),
				});
				return true;
			}
			if (deps.measuring()) {
				measured = measurement()
					? null
					: { from: { x: map.x, y: map.y }, to: { x: map.x, y: map.y } };
				gesture = null;
				return true;
			}
			const target = selectedToken();
			const grabbed = target ? handleAt(activeHandles(handleList), map.x, map.y, deps.scale()) : null;
			if (target && grabbed) {
				gesture = {
					kind: "shape",
					id: target.id,
					handle: { ...grabbed },
					width: target.width,
					height: target.height,
					rotation: target.rotation,
					moved: false,
				};
				return true;
			}
			const hit = hitTest(state.pawns, deps.viewed(), grid(), map.x, map.y, concealed);
			if (hit && mayMove(hit, role, user)) {
				gesture = {
					kind: "press",
					anchor: hit.id,
					screen: { x: screen.x, y: screen.y },
					grab: { x: hit.x - map.x, y: hit.y - map.y },
					mods,
				};
				return true;
			}
			gesture = {
				kind: "marquee",
				from: { x: map.x, y: map.y },
				to: { x: map.x, y: map.y },
				screen: { x: screen.x, y: screen.y },
				moved: false,
				anchor: hit?.id ?? null,
			};
			return true;
		},
		drag(map, screen, mods) {
			pointer = { x: map.x, y: map.y };
			aim(map);
			if (fogging) {
				deps.fog?.drag(map, mods);
				return;
			}
			if (inking) {
				deps.draw?.drag(map, mods);
				return;
			}
			if (gesture?.kind === "press") {
				if (!past(gesture.screen, screen)) {
					return;
				}
				beginDrag(gesture);
			}
			const active = gesture;
			if (!active) {
				return;
			}
			switch (active.kind) {
				case "drag":
					moveDrag(active, map);
					return;
				case "shape":
					moveShape(active, map, mods);
					return;
				case "marquee":
					if (!active.moved && !past(active.screen, screen)) {
						return;
					}
					active.moved = true;
					active.to.x = map.x;
					active.to.y = map.y;
					announce();
					return;
			}
		},
		release(map, screen, mods) {
			pointer = { x: map.x, y: map.y };
			if (fogging) {
				fogging = false;
				deps.fog?.release(map, mods);
				return;
			}
			if (inking) {
				inking = false;
				deps.draw?.release(map, mods);
				return;
			}
			const active = gesture;
			gesture = null;
			if (!active) {
				return;
			}
			const clicked = clickedPawn(active);
			const twice = countClick(clicked?.id ?? null);
			switch (active.kind) {
				case "drag":
					commit(active, false);
					return;
				case "shape":
					commitShape(active, false);
					return;
				case "press": {
					if (!clicked) {
						return;
					}
					if (mods.shift) {
						selection.toggle(clicked.id);
					} else {
						selection.set([clicked.id]);
					}
					announce();
					if (twice && !mods.shift) {
						deps.details(clicked);
					}
					return;
				}
				case "marquee": {
					if (!active.moved) {
						if (!mods.shift && selection.clear()) {
							announce();
						}
						if (twice && !mods.shift && clicked) {
							deps.details(clicked);
						}
						return;
					}
					const rect = marqueeRect(active);
					const found = marqueeSelect(state.pawns, deps.viewed(), rect, role, user, concealed);
					if (mods.shift) {
						selection.add(found);
					} else {
						selection.set(found);
					}
					announce();
					return;
				}
			}
		},
		cancel() {
			if (fogging) {
				fogging = false;
				deps.fog?.abandon();
				return;
			}
			if (inking) {
				inking = false;
				deps.draw?.abandon();
				return;
			}
			const active = gesture;
			gesture = null;
			countClick(null);
			if (active?.kind === "drag") {
				commit(active, true);
				return;
			}
			if (active?.kind === "shape") {
				commitShape(active, true);
				return;
			}
			announce();
		},
		secondary(map, screen) {
			if (deps.fog?.secondary()) {
				fogging = false;
				return;
			}
			if (deps.draw?.secondary()) {
				inking = false;
				return;
			}
			if (abandon()) {
				return;
			}
			const hit = hitTest(state.pawns, deps.viewed(), grid(), map.x, map.y, concealed);
			if (hit) {
				deps.menu(hit, { x: screen.x, y: screen.y });
			}
		},
		hover(map) {
			pointer = map ? { x: map.x, y: map.y } : null;
			deps.fog?.hover(map);
			deps.draw?.hover(map);
			if (map) {
				aim(map);
			}
			const found = map ? hitTest(state.pawns, deps.viewed(), grid(), map.x, map.y, concealed) : null;
			const next = found?.id ?? null;
			if (next !== hovered) {
				hovered = next;
				announce();
			}
		},
		active() {
			return gesture !== null || armed !== null || previews.size > 0;
		},
	};
	function marqueeRect(active: Marqueeing): Rect {
		return { x1: active.from.x, y1: active.from.y, x2: active.to.x, y2: active.to.y };
	}
	function ghostOf(p: Ghostable, x: number, y: number, out: Drawn[], count: number): number {
		const slot = out[count] ?? (out[count] = blankDrawn());
		slot.id = p.id;
		slot.kind = p.kind;
		slot.name = p.name;
		slot.image = p.image;
		slot.x = x;
		slot.y = y;
		slot.z = p.z;
		slot.size = p.size;
		slot.width = p.width;
		slot.height = p.height;
		slot.rotation = p.rotation;
		slot.hidden = false;
		slot.health = null;
		return count + 1;
	}
	return {
		tool,
		selection,
		focus: described,
		ghosts(out) {
			expirePreviews(previews, performance.now());
			let count = 0;
			if (gesture?.kind === "drag") {
				const origin = gesture.origins.get(gesture.anchor);
				if (origin) {
					const dx = gesture.ghost.x - origin.x;
					const dy = gesture.ghost.y - origin.y;
					for (const id of gesture.ids) {
						const p = pawn(id);
						const from = gesture.origins.get(id);
						if (p && from) {
							count = ghostOf(p, from.x + dx, from.y + dy, out, count);
						}
					}
				}
			}
			for (const preview of previews.values()) {
				for (const at of preview.positions) {
					const p = pawn(at.id);
					if (p && onFloor(p)) {
						count = ghostOf(p, at.x, at.y, out, count);
					}
				}
			}
			if (gesture?.kind === "shape" && gesture.moved) {
				const p = pawn(gesture.id);
				if (p && onFloor(p)) {
					proposal.id = p.id;
					proposal.kind = p.kind;
					proposal.name = p.name;
					proposal.image = p.image;
					proposal.z = p.z;
					proposal.size = p.size;
					proposal.width = gesture.width;
					proposal.height = gesture.height;
					proposal.rotation = gesture.rotation;
					count = ghostOf(proposal, p.x, p.y, out, count);
				}
			}
			if (armed && pointer) {
				const shape = armedShape(armed, grid().cellSize);
				const point = snapFor(shape, pointer.x, pointer.y);
				count = ghostOf(shape, point.x, point.y, out, count);
			}
			out.length = count;
			return out;
		},
		marks(out) {
			out.length = 0;
			deps.fog?.marks(out);
			deps.draw?.marks(out);
			return out;
		},
		inHand() {
			return deps.draw?.inHand() ?? null;
		},
		labels(out) {
			if (!deps.draw) {
				out.length = 0;
				return out;
			}
			return deps.draw.labels(out);
		},
		concealed,
		outlines(out) {
			let count = 0;
			const add = (o: Outline): void => {
				const slot = out[count] ?? (out[count] = blankOutline());
				Object.assign(slot, o);
				count++;
			};
			const cell = grid().cellSize;
			for (const id of selection.ids()) {
				const p = pawn(id);
				if (!p || !onFloor(p)) {
					continue;
				}
				const at = shaped(p);
				const [halfW, halfH] = pawnExtents(at, cell);
				add({
					x: p.x, y: p.y, halfW, halfH,
					color: SELECT_COLOR, alpha: 0.95, thickness: 2,
					rect: p.kind === "object", rotation: at.rotation,
				});
			}
			const line = measurement();
			if (line) {
				const radius = MEASURE_POINT * deps.scale();
				add({
					x: line.from.x, y: line.from.y, halfW: radius, halfH: radius,
					color: SELF_COLOR, alpha: 0.95, thickness: MEASURE_WIDTH,
					rect: false, rotation: 0,
				});
			}
			if (gesture?.kind === "marquee" && gesture.moved) {
				const rect = marqueeRect(gesture);
				add({
					x: (rect.x1 + rect.x2) / 2,
					y: (rect.y1 + rect.y2) / 2,
					halfW: Math.abs(rect.x2 - rect.x1) / 2,
					halfH: Math.abs(rect.y2 - rect.y1) / 2,
					color: SELECT_COLOR, alpha: 0.8, thickness: 1, rect: true, rotation: 0,
				});
			}
			const fogBox = deps.fog?.outline();
			if (fogBox) {
				add(fogBox);
			}
			const eraser = deps.draw?.outline();
			if (eraser) {
				add(eraser);
			}
			out.length = count;
			return out;
		},
		rulers(out) {
			let count = 0;
			const g = grid();
			const add = (fromX: number, fromY: number, toX: number, toY: number, color: Ruler["color"]): void => {
				const a = cellAt(g, fromX, fromY);
				const b = cellAt(g, toX, toY);
				const slot = out[count] ?? (out[count] = { cells: [], x0: 0, y0: 0, x1: 0, y1: 0, label: "", color });
				supercover(a[0], a[1], b[0], b[1], slot.cells);
				const start = cellCentre(g, a[0], a[1]);
				const end = cellCentre(g, b[0], b[1]);
				slot.x0 = start[0];
				slot.y0 = start[1];
				slot.x1 = end[0];
				slot.y1 = end[1];
				slot.label = distanceLabel(cellsMoved(b[0] - a[0], b[1] - a[1], g.diagonals) * Math.max(0, g.feetPerCell));
				slot.color = color;
				count++;
			};
			const line = measurement();
			if (line) {
				const slot = out[count] ?? (out[count] = { cells: [], x0: 0, y0: 0, x1: 0, y1: 0, label: "", color: SELF_COLOR });
				slot.cells.length = 0;
				slot.x0 = line.from.x;
				slot.y0 = line.from.y;
				slot.x1 = line.to.x;
				slot.y1 = line.to.y;
				slot.label = distanceLabel(feetBetween(line.to.x - line.from.x, line.to.y - line.from.y, g));
				slot.color = SELF_COLOR;
				count++;
			}
			if (gesture?.kind === "drag") {
				const origin = gesture.origins.get(gesture.anchor);
				if (origin) {
					add(origin.x, origin.y, gesture.ghost.x, gesture.ghost.y, SELF_COLOR);
				}
			}
			for (const preview of previews.values()) {
				const anchor = preview.positions[0];
				const p = anchor ? pawn(anchor.id) : null;
				if (anchor && p && onFloor(p)) {
					add(p.x, p.y, anchor.x, anchor.y, preview.color);
				}
			}
			out.length = count;
			return out;
		},
		handles(out) {
			return activeHandles(out);
		},
		bounds() {
			const one = described();
			const ids = selection.size > 1 ? selection.ids() : one ? [one.id] : [];
			if (ids.length === 0) {
				return null;
			}
			const cell = grid().cellSize;
			let box: Rect | null = null;
			for (const id of ids) {
				const p = pawn(id);
				if (!p || !onFloor(p)) {
					continue;
				}
				const [halfW, halfH] = boundsOf(shaped(p), cell);
				if (!box) {
					box = { x1: p.x - halfW, y1: p.y - halfH, x2: p.x + halfW, y2: p.y + halfH };
					continue;
				}
				box.x1 = Math.min(box.x1, p.x - halfW);
				box.y1 = Math.min(box.y1, p.y - halfH);
				box.x2 = Math.max(box.x2, p.x + halfW);
				box.y2 = Math.max(box.y2, p.y + halfH);
			}
			return box;
		},
		preview(event) {
			switch (event.type) {
				case "pawn.dragging": {
					if (!event.by || event.by === user) {
						return;
					}
					previews.set(event.by, {
						positions: event.pawns.map((at) => ({ id: at.id, x: at.x, y: at.y })),
						color: actorColor(event.by),
						at: performance.now(),
					});
					deps.invalidate();
					return;
				}
				case "pawn.moved":
				case "pawn.updated":
				case "pawn.removed": {
					const touched = event.type === "pawn.moved"
						? event.pawns.map((at) => at.id)
						: [event.type === "pawn.updated" ? event.pawn.id : event.id];
					for (const [by, preview] of previews) {
						if (preview.positions.some((at) => touched.includes(at.id))) {
							previews.delete(by);
						}
					}
					const present = new Set(state.pawns.map((p) => p.id));
					selection.prune(present);
					if (hovered && !present.has(hovered)) {
						hovered = null;
					}
					if (event.type === "pawn.removed" && gesture?.kind === "drag") {
						if (gesture.anchor === event.id) {
							gesture = null;
						} else {
							gesture.ids = gesture.ids.filter((id) => id !== event.id);
							gesture.origins.delete(event.id);
						}
					}
					announce();
					return;
				}
				case "snapshot": {
					previews.clear();
					const present = new Set(state.pawns.map((p) => p.id));
					if (selection.prune(present)) {
						announce();
					}
					return;
				}
			}
		},
		floorChanged() {
			const present = new Set(state.pawns.filter(onFloor).map((p) => p.id));
			if (hovered && !present.has(hovered)) {
				hovered = null;
			}
			if (selection.prune(present)) {
				announce();
			}
		},
		arm,
		isArmed: () => armed !== null,
		onChange(fn) {
			changed = fn;
		},
		stop() {
			document.removeEventListener("keydown", onKeyDown);
		},
	};
	function past(from: Point, now: Point): boolean {
		const dpr = window.devicePixelRatio || 1;
		return Math.hypot(now.x - from.x, now.y - from.y) * dpr >= DRAG_THRESHOLD;
	}
}
export function hitTest(
	pawns: readonly Pawn[],
	layerID: string,
	grid: Grid,
	x: number,
	y: number,
	concealed?: (pawn: Pawn) => boolean,
): Pawn | null {
	let best: Pawn | null = null;
	for (const p of pawns) {
		if (p.layerId !== layerID) {
			continue;
		}
		if (concealed?.(p)) {
			continue;
		}
		if (best && compareStack(p, best) < 0) {
			continue;
		}
		if (containsPoint(p, x, y, grid.cellSize)) {
			best = p;
		}
	}
	return best;
}
export function expirePreviews(previews: Map<string, { at: number }>, now: number): boolean {
	let dropped = false;
	for (const [by, preview] of previews) {
		if (now - preview.at > PREVIEW_TIMEOUT) {
			previews.delete(by);
			dropped = true;
		}
	}
	return dropped;
}
function blankDrawn(): Drawn {
	return {
		id: "", kind: "monster", name: "", image: "",
		x: 0, y: 0, z: 0, size: "medium",
		width: 0, height: 0, rotation: 0, hidden: false, health: null,
	};
}
function armedShape(armed: Armed, cellSize: number): Ghostable {
	const cell = Math.max(1, cellSize);
	const object = armed.kind === "object";
	return {
		id: "armed",
		kind: armed.kind,
		name: armed.name,
		image: armed.image,
		x: 0,
		y: 0,
		z: 0,
		size: armed.size,
		width: object ? armed.width || cell : 0,
		height: object ? armed.height || cell : 0,
		rotation: 0,
	};
}
function blankOutline(): Outline {
	return {
		x: 0, y: 0, halfW: 0, halfH: 0,
		color: SELF_COLOR, alpha: 1, thickness: 1, rect: false, rotation: 0,
	};
}
