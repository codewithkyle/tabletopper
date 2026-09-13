import type { Board } from "./board.ts";
import type { Event, Pawn } from "../protocol.ts";
import type { Gesture } from "./gestures.ts";
import type { Ghostable, Handle, Overlay } from "../model/overlay.ts";
import type { Placed } from "../model/shape.ts";
import type { Point, Rect } from "../model/types.ts";
import type { Preview } from "./previews.ts";
import type { Tool } from "../render/input.ts";
import { SELECT_COLOR, SELF_COLOR, actorColor } from "../model/color.ts";
import { Selection, marqueeSelect, mayMove } from "../selection.ts";
import { blankDrawn, blankOutline, ghostOf, pool } from "../model/overlay.ts";
import { boxAround } from "./board.ts";
import { pawnExtents } from "../model/shape.ts";
import { conceals, hitTest } from "./hit.ts";
import { expirePreviews, forgetPreviews, remember, showPreviews } from "./previews.ts";
import { handleAt, handlesFor } from "../handles.ts";
import type { Pens } from "./ruler.ts";
import type { Rgb } from "../model/types.ts";
import { newPens, walkRuler } from "./ruler.ts";
import {
	beginDrag,
	clickedPawn,
	commitDrag,
	commitShape,
	marqueeRect,
	moveDrag,
	moveShape,
	newClicks,
	past,
	proposedGhost,
	shapeOf,
} from "./gestures.ts";
const START_ALPHA = 0.22;
const MARK_ALPHA = 0.3;
const OUTLINE_WIDTH = 2;
const OUTLINE_ALPHA = 0.95;
const MARQUEE_ALPHA = 0.8;
export interface TableMarks {
	open(map: Point, screen: Point): boolean;
	marked(): { x: number; y: number; size: number } | null;
}
export interface SelectDeps {
	board: Board;
	invalidate: () => void;
	scale: () => number;
	details: (pawn: Pawn) => void;
	menu: (pawn: Pawn, screen: Point) => void;
	marks?: TableMarks;
}
export interface Select extends Tool {
	selection: Selection;
	focus(): Pawn | null;
	bounds(): Rect | null;
	preview(event: Event): void;
	floorChanged(): void;
}
export function createSelect(deps: SelectDeps): Select {
	const board = deps.board;
	const selection = new Selection();
	const previews = new Map<string, Preview>();
	const concealed = conceals(board.state, board.role, board.user, board.viewed);
	const clicks = newClicks();
	const ghosts = pool(blankDrawn);
	const outlines = pool(blankOutline);
	const pens = newPens();
	const handleList: Handle[] = [];
	const shape: Placed = {
		kind: "object", size: "medium", width: 0, height: 0, rotation: 0, x: 0, y: 0,
	};
	const proposal: Ghostable = {
		id: "", kind: "object", name: "", image: "",
		x: 0, y: 0, z: 0, size: "medium", width: 0, height: 0, rotation: 0,
	};
	let gesture: Gesture = null;
	let hovered: string | null = null;
	function shaped(p: Pawn): Placed {
		return shapeOf(gesture, p, shape);
	}
	function selectedToken(): Pawn | null {
		const one = selection.only();
		const p = one ? board.pawn(one) : null;
		if (!p || p.kind !== "object" || !board.onFloor(p) || !mayMove(p, board.role, board.user)) {
			return null;
		}
		return p;
	}
	function described(): Pawn | null {
		const p = hovered ? board.pawn(hovered) : null;
		return p && p.kind !== "object" ? p : null;
	}
	function paint(
	out: Overlay, pens: Pens, centreX: number, centreY: number,
	size: number, color: Rgb, alpha: number,
): void {
	const cell = pens.cells.take();
	cell.x = centreX - size / 2;
	cell.y = centreY - size / 2;
	cell.size = size;
	cell.color = color;
	cell.alpha = alpha;
	out.cells.push(cell);
}
function activeHandles(out: Handle[]): Handle[] {
		const p = selectedToken();
		if (!p) {
			out.length = 0;
			return out;
		}
		return handlesFor(shaped(p), board.grid().cellSize, deps.scale(), out);
	}
	function under(map: Point): Pawn | null {
		return hitTest(board.state.pawns, board.viewed(), board.grid(), map.x, map.y, concealed);
	}
	function abandon(): boolean {
		const active = gesture;
		gesture = null;
		switch (active?.kind) {
			case "drag":
				commitDrag(board, active, true);
				return true;
			case "shape":
				commitShape(board, active, true);
				return true;
			case "marquee":
				board.announce();
				return true;
			default:
				return false;
		}
	}
	function box(
		out: Overlay, x: number, y: number, halfW: number, halfH: number,
		alpha: number, thickness: number, rect: boolean, rotation: number,
	): void {
		const slot = outlines.take();
		slot.x = x;
		slot.y = y;
		slot.halfW = halfW;
		slot.halfH = halfH;
		slot.color = SELECT_COLOR;
		slot.alpha = alpha;
		slot.thickness = thickness;
		slot.rect = rect;
		slot.rotation = rotation;
		out.outlines.push(slot);
	}
	return {
		press(map, screen, mods) {
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
			const hit = under(map);
			if (hit && mayMove(hit, board.role, board.user)) {
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
			if (gesture?.kind === "press") {
				if (!past(gesture.screen, screen)) {
					return;
				}
				gesture = beginDrag(board, selection, gesture);
			}
			const active = gesture;
			if (!active) {
				return;
			}
			switch (active.kind) {
				case "drag":
					moveDrag(board, active, map);
					return;
				case "shape":
					moveShape(board, active, map, mods);
					return;
				case "marquee":
					if (!active.moved && !past(active.screen, screen)) {
						return;
					}
					active.moved = true;
					active.to.x = map.x;
					active.to.y = map.y;
					board.announce();
					return;
			}
		},
		release(map, screen, mods) {
			const active = gesture;
			gesture = null;
			if (!active) {
				return;
			}
			const clicked = clickedPawn(board, active);
			const twice = clicks.count(clicked?.id ?? null);
			switch (active.kind) {
				case "drag":
					commitDrag(board, active, false);
					return;
				case "shape":
					commitShape(board, active, false);
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
					board.announce();
					if (twice && !mods.shift) {
						deps.details(clicked);
					}
					return;
				}
				case "marquee": {
					if (!active.moved) {
						if (!mods.shift && selection.clear()) {
							board.announce();
						}
						if (twice && !mods.shift && clicked) {
							deps.details(clicked);
						}
						return;
					}
					const found = marqueeSelect(
						board.state.pawns, board.viewed(), marqueeRect(active),
						board.role, board.user, concealed,
					);
					if (mods.shift) {
						selection.add(found);
					} else {
						selection.set(found);
					}
					board.announce();
					return;
				}
			}
		},
		cancel() {
			const active = gesture;
			gesture = null;
			clicks.count(null);
			if (active?.kind === "drag") {
				commitDrag(board, active, true);
				return;
			}
			if (active?.kind === "shape") {
				commitShape(board, active, true);
				return;
			}
			board.announce();
		},
		secondary(map, screen) {
			if (abandon()) {
				return true;
			}
			const hit = under(map);
			if (!hit) {
				return deps.marks?.open(map, { x: screen.x, y: screen.y }) ?? false;
			}
			deps.menu(hit, { x: screen.x, y: screen.y });
			return true;
		},
		hover(map) {
			const next = map ? under(map)?.id ?? null : null;
			if (next !== hovered) {
				hovered = next;
				board.announce();
			}
		},
		key: () => false,
		abandon,
		active: () => gesture !== null || previews.size > 0,
		contribute(out) {
			expirePreviews(previews, performance.now());
			ghosts.reset();
			outlines.reset();
			pens.reset();
			const grid = board.grid();
			if (gesture?.kind === "drag") {
				const origin = gesture.origins.get(gesture.anchor);
				if (origin) {
					const dx = gesture.ghost.x - origin.x;
					const dy = gesture.ghost.y - origin.y;
					for (const id of gesture.ids) {
						const p = board.pawn(id);
						const from = gesture.origins.get(id);
						if (p && from) {
							out.ghosts.push(ghostOf(p, from.x + dx, from.y + dy, ghosts.take()));
						}
					}
					walkRuler(out, pens, grid, origin.x, origin.y, gesture.ghost.x, gesture.ghost.y, SELF_COLOR);
				}
			}
			showPreviews(out, previews, board, pens, ghosts, concealed);
			if (gesture?.kind === "shape" && gesture.moved) {
				const p = board.pawn(gesture.id);
				if (p && board.onFloor(p)) {
					out.ghosts.push(ghostOf(proposedGhost(gesture, p, proposal), p.x, p.y, ghosts.take()));
				}
			}
			for (const id of selection.ids()) {
				const p = board.pawn(id);
				if (!p || !board.onFloor(p)) {
					continue;
				}
				const at = shaped(p);
				const [halfW, halfH] = pawnExtents(at, grid.cellSize);
				box(out, p.x, p.y, halfW, halfH, OUTLINE_ALPHA, OUTLINE_WIDTH, p.kind === "object", at.rotation);
			}
			if (gesture?.kind === "marquee" && gesture.moved) {
				const rect = marqueeRect(gesture);
				const halfW = Math.abs(rect.x2 - rect.x1) / 2;
				const halfH = Math.abs(rect.y2 - rect.y1) / 2;
				box(out, (rect.x1 + rect.x2) / 2, (rect.y1 + rect.y2) / 2, halfW, halfH, MARQUEE_ALPHA, 1, true, 0);
			}
			for (const handle of activeHandles(handleList)) {
				out.handles.push(handle);
			}
			const floor = board.state.table.layers.find((l) => l.id === board.viewed());
			if (board.role === "gm" && floor?.partyStart) {
				paint(out, pens, floor.partyStart.x, floor.partyStart.y, grid.cellSize, SELF_COLOR, START_ALPHA);
			}
			const held = deps.marks?.marked();
			if (held) {
				paint(out, pens, held.x + held.size / 2, held.y + held.size / 2, held.size, SELECT_COLOR, MARK_ALPHA);
			}
		},
		selection,
		focus: described,
		bounds() {
			const one = described();
			const ids = selection.size > 1 ? selection.ids() : one ? [one.id] : [];
			return ids.length === 0 ? null : boxAround(board, ids, shaped);
		},
		preview(event) {
			switch (event.type) {
				case "pawn.dragging": {
					if (!event.by || event.by === board.user) {
						return;
					}
					remember(previews, event.by, event.pawns, actorColor(event.by), performance.now());
					deps.invalidate();
					return;
				}
				case "pawns.moved":
				case "pawns.upserted":
				case "pawns.removed": {
					const touched = event.type === "pawns.moved"
						? event.pawns.map((at) => at.id)
						: event.type === "pawns.upserted"
						? event.pawns.map((p) => p.id)
						: event.ids;
					forgetPreviews(previews, touched);
					const present = new Set(board.state.pawns.map((p) => p.id));
					selection.prune(present);
					if (hovered && !present.has(hovered)) {
						hovered = null;
					}
					if (event.type === "pawns.removed" && gesture?.kind === "drag") {
						if (event.ids.includes(gesture.anchor)) {
							gesture = null;
						} else {
							gesture.ids = gesture.ids.filter((id) => !event.ids.includes(id));
							for (const id of event.ids) {
								gesture.origins.delete(id);
							}
						}
					}
					board.announce();
					return;
				}
				case "snapshot": {
					previews.clear();
					const present = new Set(board.state.pawns.map((p) => p.id));
					if (selection.prune(present)) {
						board.announce();
					}
					return;
				}
			}
		},
		floorChanged() {
			const present = new Set(board.state.pawns.filter(board.onFloor).map((p) => p.id));
			if (hovered && !present.has(hovered)) {
				hovered = null;
			}
			if (selection.prune(present)) {
				board.announce();
			}
		},
	};
}
