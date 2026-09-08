// What a pointer on the table means: what is under it, what a drag does, and
// what placement puts down.
//
// THIS IS A STATE MACHINE AND THE STATES ARE THE FOUR THINGS A PRESS CAN
// BECOME. A press on a movable pawn is a click until the hand has moved four
// device pixels, at which point it is a drag; a press with Shift on empty table
// is a marquee; a press on anything else is the camera's, and this only watches
// it to know whether the click that ends it should clear the selection.
//
// NOTHING HERE ALLOCATES PER FRAME. The ghosts, the outlines and the ruler are
// written into arrays the caller owns and reuses, because they are read once
// per frame for as long as a hand is moving.
//
// THE DRAG IS PREVIEWED LOCALLY AND COMMITTED ONCE. Every client works out the
// same ghosts and the same path from the same two cells, so what crosses the
// wire while a hand is moving is one position a few times a second -- and the
// move itself is a single command at the end. See decision 7.

import type { Drawn } from "./render/pawn-pass.ts";
import type { Event, Grid, Pawn, PawnKind, Role, Size, State } from "./protocol.ts";
import type { Modifiers, Tool } from "./render/input.ts";
import type { Outgoing } from "./socket.ts";
import type { Point, Rect } from "./render/camera.ts";
import { Selection, dragSet, marqueeSelect, mayMove } from "./selection.ts";
import type { Placed, Sized } from "./render/path.ts";
import {
	cellAt,
	cellCentre,
	boundsOf,
	cellsMoved,
	containsPoint,
	distanceLabel,
	pawnExtents,
	snapPawn,
	snapsToGrid,
	supercover,
} from "./render/path.ts";
import type { Handle } from "./handles.ts";
import { SPIN_STEP, handleAt, handlesFor, resized, turned } from "./handles.ts";
import { compareStack } from "./render/scene.ts";

// DRAG_THRESHOLD is how far the hand moves before a press stops being a click,
// in DEVICE pixels. Four is under a millimetre and above the jitter of a hand
// resting on a mouse -- which is what it is for, because a click that
// accidentally moved a goblin one cell is a click nobody notices until the
// fight is over.
const DRAG_THRESHOLD = 4;

// DRAG_HZ is how often a drag reports itself when snapping is OFF. With
// snapping on the hovered cell is the clock and this never runs: the hot path
// quantises itself, which is the whole reason cell-change is the trigger.
const DRAG_INTERVAL = 1000 / 20;

// PREVIEW_TIMEOUT drops somebody else's ghosts when their drag stops arriving.
// A tab that was closed mid-drag, or a connection that went away, would
// otherwise leave a ghost on everybody's table until the room reloaded.
const PREVIEW_TIMEOUT = 3000;

// GHOST_ALPHA is how solid a previewed position is. Half, so the committed
// pawn underneath is still legible -- what a drag shows is a proposal, and a
// proposal that hid what it was replacing would be worse than no preview.
export const GHOST_ALPHA = 0.5;

// ACTOR_COLORS is how one dragging player is told from another.
//
// A FIXED PALETTE INDEXED BY A HASH OF THE PLAYER ID, so the colour is stable
// across a session and across a reload without anything being stored or sent.
// Six people at a table and eight colours means a collision is possible and
// harmless: two ghosts the same colour is two ghosts, which is what they are.
const ACTOR_COLORS: readonly (readonly [number, number, number])[] = [
	[0.36, 0.65, 0.98],
	[0.99, 0.6, 0.28],
	[0.42, 0.82, 0.5],
	[0.85, 0.44, 0.9],
	[0.98, 0.78, 0.3],
	[0.4, 0.85, 0.83],
	[0.95, 0.45, 0.5],
	[0.72, 0.72, 0.78],
];

// SELF_COLOR is this client's own ruler. It is deliberately not from the
// palette: your own drag is the one you are looking at, and it reads against
// both themes.
const SELF_COLOR: readonly [number, number, number] = [0.98, 0.98, 0.99];

// Outline is one ring or rectangle for the ring pass: a selection, a ghost, or
// the marquee.
export interface Outline {
	x: number;
	y: number;
	halfW: number;
	halfH: number;
	color: readonly [number, number, number];
	alpha: number;
	thickness: number;
	rect: boolean;

	// rotation is degrees clockwise about the centre, which an outline round a
	// turned token needs and the marquee never has.
	rotation: number;
}

// Ruler is one drag's path: the cells it crosses, the line across them, and how
// far that is.
export interface Ruler {
	cells: number[];
	x0: number;
	y0: number;
	x1: number;
	y1: number;
	label: string;
	color: readonly [number, number, number];
}

// Armed is what placement will put down on the next click, built by the spawn
// dialog out of the card that was picked and the one switch above it.
//
// IT IS ONLY EVER THE GM'S. Putting something on the table is refused for
// everybody else in PawnSpawn.Authorize, and the dialog that builds this is a
// GM-only fragment, so a player's canvas is never armed.
export interface Armed {
	kind: PawnKind;
	id: string;
	name: string;
	image: string;
	visible: boolean;

	// size is the creature size the ghost is drawn at, and it is ignored for an
	// object -- which is measured in pixels below.
	size: Size;

	// width and height are the PICTURE'S own pixels for an object, and zero
	// when the library row does not record them. Zero means one cell, which is
	// the same fallback the hub applies when it resolves the spawn, so the
	// ghost and the pawn that lands are the same size either way.
	//
	// THEY ARE NOT SENT. The spawn command carries no size at all: the hub
	// reads the assets row again and takes the answer from there. These exist
	// so the thing following the pointer is the thing about to be placed.
	width: number;
	height: number;
}

// Ghostable is the part of a pawn a ghost is built from. It is a Pick rather
// than Pawn because the armed spec is not a pawn yet -- it has no id, no owner
// and no floor -- and casting one into a Pawn to draw it was a lie the compiler
// had to be talked out of.
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

	// scale is how many MAP pixels one CSS pixel covers, which is the camera's
	// zoom inverted. The resize handles are the only thing here that needs it:
	// they are a fixed size on screen and are therefore a moving size on the
	// table, and a hand grabbing one is aiming in screen pixels.
	scale: () => number;
}

export interface Table {
	tool: Tool;
	selection: Selection;

	// focus is what the overlay is about: the one selected pawn, or the hovered
	// one when nothing is selected, or nothing.
	focus(): Pawn | null;

	ghosts(out: Drawn[]): Drawn[];
	outlines(out: Outline[]): Outline[];
	rulers(out: Ruler[]): Ruler[];

	// handles is the resize and rotate controls, which exist for exactly one
	// selected object the viewer may move and are empty every other time.
	handles(out: Handle[]): Handle[];

	// bounds is the box the overlay sits above: one pawn's, or the selection's.
	bounds(): Rect | null;

	// preview consumes the events that start and end somebody else's ghosts.
	preview(event: Event): void;

	arm(armed: Armed | null): void;
	isArmed(): boolean;

	// onChange fires when the overlay's CONTENTS need rewriting, which is a
	// different question from whether the canvas needs a frame: the overlay's
	// text is set on a change and its position on every frame.
	onChange(fn: () => void): void;

	stop(): void;
}

// A press that has not yet decided what it is.
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

// Shaping is a hand on a resize or rotate handle. It carries the proposal
// rather than writing it to the pawn: the preview is a ghost, and the whole
// gesture leaves as one PawnUpdate on release.
//
// THE HANDLE IS COPIED AND NOT REFERENCED. handlesFor writes into an array it
// reuses every frame, so holding the object it handed back would be holding a
// slot that has since become a different handle.
interface Shaping {
	kind: "shape";
	id: string;
	handle: Handle;
	width: number;
	height: number;
	rotation: number;

	// moved is whether the hand actually asked for anything. A click that
	// landed on a handle and went nowhere sends nothing.
	moved: boolean;
}

interface Marqueeing {
	kind: "marquee";
	from: Point;
	to: Point;
}

// Panning is the camera's gesture, watched only so that the click which ends it
// can clear the selection.
interface Panning {
	kind: "pan";
	screen: Point;
	moved: boolean;
}

type Gesture = Pressing | Dragging | Shaping | Marqueeing | Panning | null;

// Preview is somebody else's drag, as it arrived.
interface Preview {
	positions: { id: string; x: number; y: number }[];
	color: readonly [number, number, number];
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
	let changed: (() => void) | null = null;

	// Scratch, so nothing in the frame path allocates.
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

	// THE FLOOR IS THE FILTER AND IT IS APPLIED IN ONE PLACE. Hit testing, the
	// marquee, the riders lookup and the hover all go through here, so a pawn
	// on another floor is not reachable by any of them.
	function onFloor(p: Pawn): boolean {
		return p.layerId === deps.viewed();
	}

	// shaped is a pawn as the viewer's own hand is currently proposing it: the
	// committed pawn, with a live resize or rotation written over the top.
	//
	// IT IS ONE SCRATCH OBJECT, reused, because the outline, the handles and the
	// ghost all ask for it on every frame of a gesture.
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

	// selectedToken is the one token the handles belong to, and activeHandles is
	// the controls that are up right now.
	//
	// ONE SELECTED OBJECT AND NOT A HOVERED ONE. Handles are a commitment: they
	// sit under the pointer and take a press that would otherwise have moved the
	// token, so they appear when somebody has said which token they mean.
	//
	// A MULTIPLE SELECTION HAS NONE either, because scaling six things about six
	// different centres is not one gesture, and scaling them about a shared one
	// would move five of them.
	function selectedToken(): Pawn | null {
		const one = selection.only();
		const p = one ? pawn(one) : null;

		if (!p || p.kind !== "object" || !onFloor(p) || !mayMove(p, role, user)) {
			return null;
		}

		return p;
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
			// ALT TAKES THE WAGON OUT FROM UNDER ITS RIDERS, which is the one
			// thing the automatic rule cannot express on its own.
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

	// moveDrag places the anchor's ghost and reports the drag when the cell it
	// is over has changed.
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

		// WITH SNAPPING ON THE CELL IS THE CLOCK, which is what makes the hot
		// path quantise itself: a fast mouse and a slow one cross the same
		// boundaries and send the same number of frames. With it off there is no
		// boundary to cross, so a timer stands in.
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
			others: active.ids.filter((id) => id !== active.anchor),
		});
	}

	// commit ends a drag, at the ghost or back where it started.
	//
	// A CANCELLED DRAG SENDS THE COMMITTED POSITION RATHER THAN NOTHING. The
	// server answers with pawn.moved carrying unchanged positions, and that
	// event is what tells everybody else to drop the ghosts they are drawing --
	// so Escape needs no event of its own. See decision 8.
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
			others: active.ids.filter((id) => id !== active.anchor),
		});

		announce();
	}

	// moveShape is a hand on a handle: it works out what the pointer is asking
	// for and keeps it as a proposal. Nothing is sent while it runs.
	//
	// NOBODY ELSE SEES IT HAPPEN, which is the one way this is unlike a move
	// drag. pawn.dragging carries positions and only positions, and a preview of
	// a resize would be a second hot-path event for a gesture that lasts a
	// second and happens between fights. What the others get is the result.
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

	// commitShape ends a handle drag. Unlike a move, a cancelled one sends
	// NOTHING: the proposal never left this client, so there is nothing anybody
	// else has to be told to stop drawing.
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

		// NO SIZE FOR AN OBJECT AND NO FOOTPRINT FOR ANYTHING. What an object
		// is, is the picture named by assetId, and how big it is, is a column
		// beside that picture -- so there is nothing here for a browser to say
		// about it and nothing for the server to have to distrust.
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
			assetId: armed.kind === "npc" || armed.kind === "object" ? armed.id || undefined : undefined,
			name: armed.kind === "npc" || armed.kind === "object" ? armed.name : undefined,
		});
	}

	// abandon is the one way out, and Escape and the right button are the two
	// ways to ask for it. It answers whether there was anything to abandon,
	// which is what decides whether the browser's context menu appears.
	//
	// THE ORDER MATTERS. A GM placing an encounter presses it to stop placing,
	// and a GM mid-drag presses it to put the pawn back. Placement wins, because
	// it is the mode you are IN rather than the gesture you are making.
	function abandon(): boolean {
		if (armed) {
			arm(null);

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

	function onKeyDown(e: KeyboardEvent): void {
		if (e.key === "Escape") {
			abandon();
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

			if (armed) {
				place(map);

				// THE MODE SURVIVES A SPAWN, which is the whole reason arming
				// is worth a round trip: an encounter is eight goblins and
				// eight clicks, not eight visits to a dialog.
				return true;
			}

			// A HANDLE IS TESTED BEFORE THE TABLE IS, because a handle sits ON
			// the edge of the token it belongs to: whichever of the two the
			// press is nearer, somebody aiming at a five-pixel box meant the
			// box. Only the selected object has any, so this costs nine
			// distance tests and only while something is selected.
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

			const hit = hitTest(state.pawns, deps.viewed(), grid(), map.x, map.y);

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

			if (mods.shift) {
				gesture = { kind: "marquee", from: { x: map.x, y: map.y }, to: { x: map.x, y: map.y } };

				return true;
			}

			// Empty table, or something this viewer may not move. The camera
			// takes the gesture; this watches it only so that a click which
			// went nowhere can clear the selection.
			gesture = { kind: "pan", screen: { x: screen.x, y: screen.y }, moved: false };

			return false;
		},

		drag(map, screen, mods) {
			pointer = { x: map.x, y: map.y };

			// A PRESS BECOMES A DRAG HERE AND NOWHERE ELSE, which is what keeps
			// a click from moving anything: under four device pixels this
			// returns without touching the table, and the release that follows
			// is a selection.
			if (gesture?.kind === "press") {
				if (!past(gesture.screen, screen)) {
					return;
				}

				beginDrag(gesture);
			}

			// Re-read, because beginDrag replaced it.
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
					active.to.x = map.x;
					active.to.y = map.y;
					announce();

					return;

				case "pan":
					if (past(active.screen, screen)) {
						active.moved = true;
					}

					return;
			}
		},

		release(map, screen, mods) {
			pointer = { x: map.x, y: map.y };

			const active = gesture;
			gesture = null;

			if (!active) {
				return;
			}

			switch (active.kind) {
				case "drag":
					commit(active, false);

					return;

				case "shape":
					commitShape(active, false);

					return;

				case "press": {
					// It stayed a click. Shift toggles one pawn in or out;
					// anything else selects it alone.
					const hit = pawn(active.anchor);
					if (!hit) {
						return;
					}

					if (mods.shift) {
						selection.toggle(hit.id);
					} else {
						selection.set([hit.id]);
					}
					announce();

					return;
				}

				case "marquee": {
					const rect = marqueeRect(active);
					const found = marqueeSelect(state.pawns, deps.viewed(), rect, role, user);

					if (mods.shift) {
						selection.add(found);
					} else {
						selection.set(found);
					}
					announce();

					return;
				}

				case "pan":
					if (!active.moved && selection.clear()) {
						announce();
					}

					return;
			}
		},

		cancel() {
			const active = gesture;
			gesture = null;

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

		secondary: abandon,

		hover(map) {
			pointer = map ? { x: map.x, y: map.y } : null;

			const found = map ? hitTest(state.pawns, deps.viewed(), grid(), map.x, map.y) : null;
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
		slot.dead = false;

		return count + 1;
	}

	return {
		tool,
		selection,

		focus() {
			const one = selection.only();
			if (one) {
				return pawn(one);
			}
			if (selection.size > 0) {
				return null;
			}

			return hovered ? pawn(hovered) : null;
		},

		ghosts(out) {
			// A DRAG THAT STOPPED ARRIVING IS DROPPED HERE, on the frame that
			// reads it. A tab closed mid-drag, or a connection that went away,
			// would otherwise leave a ghost on everybody's table for the rest of
			// the session -- and, because a live preview is one of the things
			// that keeps the frame loop awake, it would leave every client
			// rendering for ever to draw it.
			expirePreviews(previews, performance.now());

			let count = 0;

			// This client's own drag.
			if (gesture?.kind === "drag") {
				const origin = gesture.origins.get(gesture.anchor);
				if (origin) {
					const dx = gesture.ghost.x - origin.x;
					const dy = gesture.ghost.y - origin.y;

					for (const id of gesture.ids) {
						const p = pawn(id);
						const from = gesture.origins.get(id);
						if (p && from) {
							// ONE DELTA FOR EVERYBODY AND ONLY THE ANCHOR
							// SNAPS. A wagon with three people on it must arrive
							// with them sitting in the same three spots, and
							// snapping each one on its own would shuffle them
							// into the wagon's cells. The server applies the
							// same rule to the committed move.
							count = ghostOf(p, from.x + dx, from.y + dy, out, count);
						}
					}
				}
			}

			// Everybody else's.
			for (const preview of previews.values()) {
				for (const at of preview.positions) {
					const p = pawn(at.id);
					if (p && onFloor(p)) {
						count = ghostOf(p, at.x, at.y, out, count);
					}
				}
			}

			// A RESIZE OR A ROTATION IN PROGRESS, drawn as the same kind of
			// proposal a move is: the committed token stays where it is and the
			// ghost shows what letting go would do.
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

			// And the thing placement is armed with, following the pointer.
			if (armed && pointer) {
				const shape = armedShape(armed, grid().cellSize);
				const point = snapFor(shape, pointer.x, pointer.y);
				count = ghostOf(shape, point.x, point.y, out, count);
			}

			out.length = count;

			return out;
		},

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

				// THE OUTLINE FOLLOWS THE PROPOSAL AND NOT THE PAWN, so a hand
				// on a corner handle sees the box it is about to get rather
				// than the one it started with.
				const at = shaped(p);
				const [halfW, halfH] = pawnExtents(at, cell);

				add({
					x: p.x, y: p.y, halfW, halfH,
					color: SELF_COLOR, alpha: 0.95, thickness: 2,
					rect: p.kind === "object", rotation: at.rotation,
				});
			}

			if (gesture?.kind === "marquee") {
				const rect = marqueeRect(gesture);
				add({
					x: (rect.x1 + rect.x2) / 2,
					y: (rect.y1 + rect.y2) / 2,
					halfW: Math.abs(rect.x2 - rect.x1) / 2,
					halfH: Math.abs(rect.y2 - rect.y1) / 2,
					color: SELF_COLOR, alpha: 0.8, thickness: 1, rect: true, rotation: 0,
				});
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

			if (gesture?.kind === "drag") {
				const origin = gesture.origins.get(gesture.anchor);
				if (origin) {
					add(origin.x, origin.y, gesture.ghost.x, gesture.ghost.y, SELF_COLOR);
				}
			}

			for (const preview of previews.values()) {
				// THE ANCHOR IS THE FIRST POSITION, because that is the order
				// the server builds the list in -- selection() puts the anchor
				// at the head. It is a convention rather than a field, and the
				// one case it is wrong is a hidden wagon carrying a visible
				// rider, where a player's projected list starts with the rider.
				// The path is then drawn for the wrong pawn of the group, which
				// is a wrong line rather than a wrong move.
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
			const ids = selection.size > 0 ? selection.ids() : hovered ? [hovered] : [];
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

				// THE SCREEN-ALIGNED BOX AND NOT THE TOKEN'S OWN. What sits on
				// this is the DOM overlay, which is a rectangle on the page: a
				// long token turned on its side is wide here and tall in its
				// own frame, and an overlay placed on the second would land
				// across the middle of it.
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

				// A COMMITTED MOVE IS WHAT ENDS A PREVIEW, which is why a
				// cancelled drag sends one rather than an event of its own.
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

					// A pawn that left the table cannot stay selected: its next
					// drag would be answered not_found.
					const present = new Set(state.pawns.map((p) => p.id));
					selection.prune(present);
					if (hovered && !present.has(hovered)) {
						hovered = null;
					}

					// AND THE OVERLAY IS REWRITTEN WHATEVER CHANGED, because
					// what it says about a pawn -- its hit points, its
					// conditions, its name -- moved even when the selection did
					// not. These three are the committed events and arrive at
					// human pace; pawn.dragging, which does not, is handled
					// above and announces nothing.
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
		// The threshold is in device pixels and the pointer is in CSS pixels,
		// which on a retina screen are not the same distance.
		const dpr = window.devicePixelRatio || 1;

		return Math.hypot(now.x - from.x, now.y - from.y) * dpr >= DRAG_THRESHOLD;
	}
}

// hitTest is what is under a point, topmost first.
//
// ON THE CPU, IN ONE PASS, WITH NO SORT. A few hundred pawns is a few
// microseconds, and the topmost is found by keeping the best rather than by
// ordering the whole table -- which would allocate on every pointer move.
//
// A DISC FOR A CREATURE AND A RECTANGLE FOR AN OBJECT, turned with the token,
// which is containsPoint and is exactly what the pawn pass drew: a click on the
// corner of a wagon's bounding box hits the wagon, a click on the corner of a
// goblin's does not hit the goblin, and a click just off the corner of a token
// turned forty degrees hits the table.
//
// TOPMOST IS compareStack AND NOT z. Every token is drawn under every creature,
// so clicking where a goblin stands on a rug picks the goblin -- the rug is not
// reachable there, because it is not what is on top there. It is the same
// comparison the draw order uses, which is the whole point: what a click picks
// is what a person can see.
export function hitTest(
	pawns: readonly Pawn[],
	layerID: string,
	grid: Grid,
	x: number,
	y: number,
): Pawn | null {
	let best: Pawn | null = null;

	for (const p of pawns) {
		if (p.layerId !== layerID) {
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

// actorColor is a stable colour per player, from a hash of their id. Nothing is
// stored and nothing is sent: two clients compute the same answer because they
// are looking at the same ULID.
export function actorColor(id: string): readonly [number, number, number] {
	let hash = 0;
	for (let i = 0; i < id.length; i++) {
		hash = (hash * 31 + id.charCodeAt(i)) >>> 0;
	}

	return ACTOR_COLORS[hash % ACTOR_COLORS.length] ?? ACTOR_COLORS[0];
}

// expired drops previews nobody has updated. It is exported so the frame can
// call it without this module owning a timer -- a timer would keep a tab awake
// that has nothing to draw.
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
		width: 0, height: 0, rotation: 0, hidden: false, dead: false,
	};
}

// armedShape is the armed spec as something that can be snapped, measured and
// drawn. An object with no recorded picture size is one cell, which is what the
// hub places it at.
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

		// A TOKEN GOES DOWN SQUARE. There is no angle on the wire for a spawn
		// and none in the dialog; it is turned afterwards, with the handles.
		rotation: 0,
	};
}

function blankOutline(): Outline {
	return {
		x: 0, y: 0, halfW: 0, halfH: 0,
		color: SELF_COLOR, alpha: 1, thickness: 1, rect: false, rotation: 0,
	};
}
