// THE CAMERA FOLLOWS THE TURN. Notice the acting line change, work out which
// pawns that line stands for, and hand the renderer the box they fit in.
//
// IT IS A SETTING AND IT IS THE ACCOUNT'S, so this module is mounted only for a
// viewer who asked for it -- see main.ts, which reads the attribute the room
// page rendered off the session. There is no switch in here and no default: a
// module that is not mounted is the whole of "off", which costs nothing and
// cannot be half on.
//
// IT COMPARES THE ACTIVE ID RATHER THAN REACTING TO THE EVENT, for the reason
// the turn clock does. initiative.updated is raised by every change to the
// tracker -- a line added, a line removed, a drag that reorders the strip, a
// sync that rebuilds it -- and none of those moved the turn. Only the id in
// state.initiative.active changing is a creature being handed the table.
//
// A SNAPSHOT IS NOT A TURN CHANGE, which is the same rule the blood follows in
// main.ts. It arrives on the first join and on every reconnect, and what it
// carries is the fight as it stands NOW -- so a laptop opened mid-combat, or a
// tab that was asleep through three rounds, would come back and yank the camera
// somewhere on the strength of a turn nobody at this browser watched begin.
// What a snapshot does instead is record where the fight has got to, so the
// NEXT turn is followed from the right place.
//
// A LINE WITH NO PAWNS MOVES NOTHING. "Lair action" and "the volcano erupts"
// are entries in the order with no creature on the table, which is exactly what
// the empty pawn list on an entry is for, and there is nowhere to point a camera
// at for one of them. Neither is a line whose pawns are all on another floor:
// which floor a GM is looking at is their own local choice -- they are usually
// preparing the next room -- and taking that away mid-fight would be a worse
// theft than the one this feature is trying to prevent. A player only ever has
// the active floor's pawns at all, so for them the case cannot arise.

import type { Event, State } from "./protocol.ts";
import type { Rect } from "./render/camera.ts";
import { boundsOf } from "./render/path.ts";

export interface Follow {
	// event is every event off the socket, filtered in here rather than by the
	// caller. It is the shape table.preview takes for the same reason: what
	// counts as a turn moving is this module's business and not main.ts's.
	event(event: Event): void;
}

export interface FollowOptions {
	// viewed is the floor this browser is looking at, which the renderer owns
	// -- the GM may be looking at one that is not active, and nothing outside
	// the renderer knows that.
	viewed(): string;

	// focus is the camera move. It takes a box in map pixels and eases onto it;
	// see the renderer.
	focus(rect: Rect): void;
}

export function mountFollow(state: State, options: FollowOptions): Follow {
	// seen is the acting line this browser last knew about. It is separate from
	// the store because the store holds WHICH line is acting and this holds
	// whether that is news.
	let seen: string | null = null;

	function event(e: Event): void {
		if (e.type === "snapshot") {
			seen = state.initiative.active;

			return;
		}

		if (e.type !== "initiative.updated") {
			return;
		}

		const active = state.initiative.active;
		if (active === seen) {
			return;
		}

		seen = active;

		const box = actingBounds(state, options.viewed());
		if (box) {
			options.focus(box);
		}
	}

	return { event };
}

// actingBounds is the box the acting line occupies on the floor being looked
// at, and null when there is nothing there to look at.
//
// A GROUP IS BOXED BY EVERY ONE OF ITS PAWNS, which is what grouped combat
// means: nine goblins act on one count, so the turn belongs to all nine and a
// camera that framed the first of them would leave eight of the creatures whose
// turn it is off the screen. The renderer is what zooms out to hold the result.
//
// A HIDDEN PAWN IS IN THE BOX, and only the GM ever has one -- the projection in
// internal/room removes them before a player's events are encoded. An ambush
// that is acting is precisely what the GM needs to be looking at, and leaving it
// out would frame the visible half of a fight.
//
// IT IS ONE PASS OVER THE PAWNS AND NOT ONE LOOKUP PER ID, because the store
// holds an array and there is no index on it. A turn change is not a hot path.
export function actingBounds(state: State, viewed: string): Rect | null {
	const active = state.initiative.active;
	if (active === null) {
		return null;
	}

	const entry = state.initiative.entries.find((line) => line.id === active);
	if (!entry || entry.pawnIds.length === 0) {
		return null;
	}

	const cell = state.table.grid.cellSize;
	let box: Rect | null = null;

	for (const pawn of state.pawns) {
		if (pawn.layerId !== viewed || !entry.pawnIds.includes(pawn.id)) {
			continue;
		}

		// THE SCREEN-ALIGNED BOX AND NOT THE TOKEN'S OWN, which is the same
		// choice the overlay makes: a long token turned on its side is wide on
		// the map and tall in its own frame, and a camera framed against the
		// second would cut the ends off it.
		const [halfW, halfH] = boundsOf(pawn, cell);

		if (!box) {
			box = { x1: pawn.x - halfW, y1: pawn.y - halfH, x2: pawn.x + halfW, y2: pawn.y + halfH };

			continue;
		}

		box.x1 = Math.min(box.x1, pawn.x - halfW);
		box.y1 = Math.min(box.y1, pawn.y - halfH);
		box.x2 = Math.max(box.x2, pawn.x + halfW);
		box.y2 = Math.max(box.y2, pawn.y + halfH);
	}

	return box;
}
