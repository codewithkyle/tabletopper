// The fan-out: what happens to an event after the store has reduced it.
//
// ONE FIXED LIST OF SUBSCRIBERS, NOT A LADDER OF BRANCHES. main.ts used to be a
// sequence of `if (event.type === ...)` tests, one per family the canvas, the
// panels, the turn clock and the camera each cared about, and every phase grew
// it by a branch. Here each subsystem is handed the whole event and decides for
// itself; a new family is a new entry in the list rather than a new branch in
// somebody else's function. The order of the list is still meaning -- the
// reducer runs first and the navigation runs last -- and it is written down
// once, in main.ts.

import type { Event } from "./protocol.ts";

export type Effect = (event: Event) => void;

// fanOut runs every effect on every event, in order.
export function fanOut(effects: readonly Effect[]): Effect {
	return (event) => {
		for (const effect of effects) {
			effect(event);
		}
	};
}

// touchesPawns is which events move something the pawn pass has already put in
// its buffer. A snapshot replaces the whole table; the pawn family is itself;
// table.updated carries the grid, whose cell size is every pawn's radius.
//
// THE FOG FAMILY IS IN HERE AND IT IS NOT OBVIOUS WHY. Fog moves no pawn. What
// it moves is which pawns a PLAYER is drawn at all: a creature under the cover
// is left out of the buffer entirely rather than painted over, so that the hover
// label and the marquee cannot find what the screen does not show. That makes
// uncovering a room a change to the buffer, and without this line the goblins in
// it stay invisible until something else happens to rebuild it -- which, in the
// bug this line fixes, was the next time anybody moved a pawn.
//
// table.updated already covered the two FLAGS, which is why the first shape
// drawn on a sleeping floor looked right and every one after it did not: waking
// the floor emits a table.updated and the rest emit fog.added alone.
//
// pawn.dragging is deliberately absent. It is a preview nobody has committed
// to, it is drawn from a buffer of its own, and it is one of the three hot paths
// in the protocol -- rebuilding the whole table's instances twenty times a
// second for a ghost is exactly what the split between the two buffers exists to
// avoid.
//
// IT LIVES HERE RATHER THAN IN main.ts, where it was written, because main.ts
// runs on import: it reads the document at module scope, so nothing in it can be
// unit tested. This is a pure predicate over event names and belongs beside the
// fan-out it is asked inside.
export function touchesPawns(type: Event["type"]): boolean {
	if (type === "snapshot" || type === "table.updated" || type.startsWith("fog.")) {
		return true;
	}

	return type.startsWith("pawn.") && type !== "pawn.dragging";
}

export interface RefusalDeps {
	// alert opens the alert modal with the server's own heading and message.
	alert(heading: string, message: string): void;

	// resync asks for the whole room again, which is what a not_found refusal
	// calls for: the client is holding something the server no longer has.
	resync(): void;
}

// refusals is what the client does with an error frame, and until this existed
// it did nothing: the frame arrived, carried a heading and a message written
// for the person reading it, and was discarded. A player dragging a pawn the
// GM had just hidden watched the ghost vanish and the pawn sit still, three
// times, and concluded drag was broken.
//
// not_found IS THE ONE CODE THAT SAYS NOTHING. It is nobody's mistake -- two
// people removing the same goblin is a race the table produces on its own --
// and the answer is a fresh snapshot, which prunes the selection and ends any
// gesture that named the missing thing. Everything else is shown.
export function refusals(deps: RefusalDeps): Effect {
	return (event) => {
		if (event.type !== "error") {
			return;
		}

		if (event.code === "not_found") {
			deps.resync();

			return;
		}

		deps.alert(event.heading, event.message);
	};
}
