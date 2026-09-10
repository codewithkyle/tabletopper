// Which pawns the viewer has picked out, and what goes with them when one is
// dragged.
//
// SELECTION IS LOCAL AND NOTHING ABOUT IT IS ON THE WIRE. Nobody else needs to
// know what a GM has highlighted, there is no state to converge, and a protocol
// that carried it would have to answer what happens when two people select the
// same goblin. What IS on the wire is the move it produces.
//
// IT HOLDS ONLY WHAT THE VIEWER MAY MOVE. A player who marquees across a room
// full of goblins and their own fighter selects the fighter, because a
// selection that included the goblins would be a selection whose every drag was
// refused -- the server checks the same rule again on receipt, and a client that
// let somebody build a selection the server would reject is a client that
// teaches them the wrong thing about their own table.
//
// THE ORDER IS INSERTION ORDER, which matters for one thing: the pawn the
// overlay names when several are selected, and the order ids reach the server.
// A Set preserves it and that is the whole reason this is one.

import type { Pawn, Role } from "./protocol.ts";
import type { Rect } from "./render/camera.ts";
import { compareStack } from "./render/scene.ts";
import { pawnExtents } from "./render/path.ts";

// SELECTION_MAX is room.SelectionMax: what one move, drag or remove may carry.
// It is written here rather than imported from the generated protocol because
// the generator emits types and not limits; a test pins the two together.
export const SELECTION_MAX = 200;

// mayMove is the authority rule, and it is the client's copy of
// State.requireControl: the GM may move anything on the table, and a player may
// move what they own.
//
// A PLAYER'S STORE HOLDS NOTHING THEY CANNOT SEE, so the "and can see it" half
// of the server's rule needs no code here -- a pawn that is hidden or on
// another floor never reached them.
export function mayMove(pawn: Pawn, role: Role, user: string): boolean {
	return role === "gm" || (pawn.ownerId !== null && pawn.ownerId === user);
}

// Selection is an ordered set of ids, and every mutator answers whether
// anything actually changed -- which is what the overlay redraws on and the
// canvas asks for a frame on.
export class Selection {
	private readonly chosen = new Set<string>();

	has(id: string): boolean {
		return this.chosen.has(id);
	}

	get size(): number {
		return this.chosen.size;
	}

	ids(): string[] {
		return [...this.chosen];
	}

	// only is the single id when exactly one is selected, which is the case the
	// overlay draws a whole pawn for.
	only(): string | null {
		return this.chosen.size === 1 ? (this.chosen.values().next().value ?? null) : null;
	}

	set(ids: readonly string[]): boolean {
		const next = ids.slice(0, SELECTION_MAX);
		if (this.same(next)) {
			return false;
		}

		this.chosen.clear();
		for (const id of next) {
			this.chosen.add(id);
		}

		return true;
	}

	add(ids: readonly string[]): boolean {
		let changed = false;

		for (const id of ids) {
			if (this.chosen.size >= SELECTION_MAX || this.chosen.has(id)) {
				continue;
			}
			this.chosen.add(id);
			changed = true;
		}

		return changed;
	}

	toggle(id: string): boolean {
		if (this.chosen.delete(id)) {
			return true;
		}
		if (this.chosen.size >= SELECTION_MAX) {
			return false;
		}

		this.chosen.add(id);

		return true;
	}

	clear(): boolean {
		if (this.chosen.size === 0) {
			return false;
		}

		this.chosen.clear();

		return true;
	}

	// prune drops ids that are no longer on the table, which is a pawn removed,
	// hidden, or walked up a staircase. Without it a selection would go on
	// naming pawns whose next drag the server answers not_found.
	prune(present: ReadonlySet<string>): boolean {
		let changed = false;

		for (const id of this.chosen) {
			if (!present.has(id)) {
				this.chosen.delete(id);
				changed = true;
			}
		}

		return changed;
	}

	private same(ids: readonly string[]): boolean {
		if (ids.length !== this.chosen.size) {
			return false;
		}

		for (const id of ids) {
			if (!this.chosen.has(id)) {
				return false;
			}
		}

		return true;
	}
}

// marqueeSelect is every movable pawn on the viewed floor whose CENTRE is
// inside the rectangle.
//
// THE CENTRE AND NOT THE FOOTPRINT, which is the choice a dragged box has to
// make and the one that behaves. Intersection would put a gargantuan dragon
// into a box drawn between its toes; containment would make a box drawn across
// the middle of a battle line select nothing at all. The centre is where the
// pawn IS, and it is what somebody dragging a box is aiming at.
export function marqueeSelect(
	pawns: readonly Pawn[],
	layerID: string,
	rect: Rect,
	role: Role,
	user: string,
	concealed?: (pawn: Pawn) => boolean,
): string[] {
	const x1 = Math.min(rect.x1, rect.x2);
	const x2 = Math.max(rect.x1, rect.x2);
	const y1 = Math.min(rect.y1, rect.y2);
	const y2 = Math.max(rect.y1, rect.y2);

	const found: string[] = [];

	for (const pawn of pawns) {
		if (pawn.layerId !== layerID || !mayMove(pawn, role, user)) {
			continue;
		}

		// AND A BOX DRAGGED ACROSS THE FOG PICKS UP NOTHING UNDER IT. This one
		// is the least visible of the four concealment gates and the most
		// telling: a player who swept an empty-looking corridor and found four
		// goblins in their selection has been told where the goblins are.
		if (concealed?.(pawn)) {
			continue;
		}
		if (pawn.x < x1 || pawn.x > x2 || pawn.y < y1 || pawn.y > y2) {
			continue;
		}

		found.push(pawn.id);
		if (found.length >= SELECTION_MAX) {
			break;
		}
	}

	return found;
}

// riders is what a wagon carries.
//
// THERE IS NO ATTACH RELATIONSHIP AND THERE IS NOT GOING TO BE ONE. Standing on
// something is already expressed by the table: the pawn is inside the wagon's
// footprint, and it is above the wagon in draw order because it was spawned
// after it. Both of those are facts the state already holds, so "who is on the
// wagon" is a question rather than a stored answer -- which means it cannot go
// stale, and a player who steps off is off.
//
// IT NEVER TRIGGERS ON A ONE-CELL THING, which is the rule that keeps two
// goblins in adjacent squares from picking each other up. Something that
// carries passengers is at least two cells across on one of its axes.
//
// THE GATE IS MEASURED IN PIXELS AND COMPARED AGAINST TWO CELLS, which is one
// question rather than two: a large creature is exactly two cells wide and a
// wagon is however wide the picture is. Asking each kind in its own units would
// be the same rule written twice.
//
// THE BOX IS NOT TURNED WITH THE TOKEN. A rotated wagon's riders are found in
// the box it would occupy square-on, which is generous at the corners of a
// token turned forty-five degrees -- and generous is the right direction to be
// wrong in: picking up somebody who was standing beside the cart is a drag they
// can see and undo, and leaving somebody sitting IN it behind is a rider left
// in the road.
export function riders(pawns: readonly Pawn[], anchor: Pawn, cellSize: number): string[] {
	const cell = Math.max(1, cellSize);
	const [halfW, halfH] = pawnExtents(anchor, cellSize);

	if (halfW * 2 < cell * 2 && halfH * 2 < cell * 2) {
		return [];
	}

	const found: string[] = [];

	for (const pawn of pawns) {
		// ABOVE IT IN THE DRAW ORDER, which is not the same question as a
		// higher z now that every token is drawn under every creature. A goblin
		// spawned before the wagon still stands ON it, because that is what the
		// table shows.
		if (pawn.id === anchor.id || pawn.layerId !== anchor.layerId || compareStack(pawn, anchor) <= 0) {
			continue;
		}
		if (Math.abs(pawn.x - anchor.x) > halfW || Math.abs(pawn.y - anchor.y) > halfH) {
			continue;
		}

		found.push(pawn.id);
	}

	return found;
}

// dragSet is everything that moves when the anchor does, the anchor first.
//
// THE SELECTION COMES ALONG ONLY IF THE ANCHOR IS IN IT, which is what makes
// dragging one pawn out of a selected group possible: grabbing something that
// was not selected means you meant that thing, and the selection is left alone.
//
// THE RIDERS COME ALONG WHETHER OR NOT ANYTHING WAS SELECTED, because a wagon
// with three people on it is one object as far as a hand is concerned. Alt is
// how you take the wagon out from under them.
export function dragSet(
	pawns: readonly Pawn[],
	anchor: Pawn,
	selection: Selection,
	options: { withRiders: boolean; role: Role; user: string; cellSize: number },
): string[] {
	const ids: string[] = [anchor.id];
	const seen = new Set<string>([anchor.id]);

	const take = (id: string): void => {
		if (seen.has(id) || ids.length >= SELECTION_MAX) {
			return;
		}

		const pawn = pawns.find((p) => p.id === id);
		if (!pawn || !mayMove(pawn, options.role, options.user)) {
			return;
		}

		seen.add(id);
		ids.push(id);
	};

	if (selection.has(anchor.id)) {
		for (const id of selection.ids()) {
			take(id);
		}
	}

	if (options.withRiders) {
		for (const id of riders(pawns, anchor, options.cellSize)) {
			take(id);
		}
	}

	return ids;
}
