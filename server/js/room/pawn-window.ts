// What a pawn's window IS, in one place.
//
// FOUR THINGS OPEN IT AND THEY MUST ALL OPEN THE SAME ONE. The overlay's
// Details button, a right click on the table, a rename arriving over the
// socket, and the removal that closes it: two of those build a window and two
// of them aim an id at one that is already open, so an id spelled differently
// in any of the four is a window that opens twice and never closes.
//
// IT IS KEYED BY THE PAWN AND NOT BY THE URL, which is the rule every window
// follows: the URL carries the room, and keying on it would give one GM a
// separate remembered position per table for the same panel.
//
// THE SIZE IS HERE RATHER THAN IN THE MARKUP because this panel is the pawn's
// whole editor -- a name, a size, hit points, armour class, conditions,
// visibility and a floor -- and a window that opened at the size of a read-only
// summary would need dragging larger before it could be used. It applies the
// first time somebody opens one; after that the size they left it at wins.

import type { WindowSpec } from "./window.ts";

export const PAWN_WINDOW = "pawn:";

const WIDTH = 320;
const HEIGHT = 520;

// Named is the part of a pawn this needs, which is its identity and its
// heading. Taking the whole Pawn would make every caller hold one, and the
// overlay's is already narrowed.
export interface Named {
	id: string;
	name: string;
}

export function pawnWindow(roomID: string, pawn: Named): WindowSpec {
	return {
		id: PAWN_WINDOW + pawn.id,
		url: `/fragment/room/pawn?room=${roomID}&pawn=${pawn.id}`,
		title: pawn.name,
		width: WIDTH,
		height: HEIGHT,
	};
}
