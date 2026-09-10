// THE CAMERA FOLLOWS THE TURN. Notice the acting line change, work out which
// pawns that line stands for, and hand the renderer the box they fit in.
//
// IT IS A SETTING AND IT IS THE ACCOUNT'S. main.ts reads the attribute the room
// page rendered off the session and hands it to following below.
//
// IT USED TO BE OFF BY NOT BEING MOUNTED, and that was better right up until the
// setting could be changed without leaving the table. Settings in the room's
// Help menu opens the account dialog over the tabletop, so the answer can arrive
// mid-fight -- and a module mounted at that moment would have no idea which turn
// it had already missed. The first initiative.updated after it woke up would
// find `seen` empty, decide the turn had moved, and yank the camera onto a
// creature whose go began five minutes ago.
//
// SO IT IS ALWAYS MOUNTED AND IT ALWAYS WATCHES. What `on` gates is the camera
// move and nothing else: the acting line is tracked whether or not anybody
// asked to be taken to it, which is what makes turning the setting on in the
// middle of a round do nothing at all until the NEXT turn -- the correct amount
// of nothing.
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
import { actingPawnIds } from "./render/scene.ts";
import { boundsOf } from "./render/path.ts";

export interface Follow {
	// event is every event off the socket, filtered in here rather than by the
	// caller. It is the shape table.preview takes for the same reason: what
	// counts as a turn moving is this module's business and not main.ts's.
	event(event: Event): void;

	// following is the account setting, set from the page on load and again
	// whenever the settings dialog is saved over the table. See the note at the
	// top for why this is a switch rather than a mount.
	following(on: boolean): void;
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

	// on is the setting. It starts true because that is the column's default,
	// and main.ts says otherwise before a single event has arrived.
	let on = true;

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

		// AND THE SETTING IS READ HERE, AFTER seen HAS BEEN UPDATED. Everything
		// above this line is bookkeeping about where the fight has got to, and
		// it is done whether or not anybody wants the camera moved -- see the
		// note at the top of the file.
		if (!on) {
			return;
		}

		const box = actingBounds(state, options.viewed());
		if (box) {
			options.focus(box);
		}
	}

	function following(next: boolean): void {
		on = next;
	}

	return { event, following };
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
	// WHICH CREATURES ARE ACTING IS NOT THIS MODULE'S QUESTION. The canvas has
	// to answer it on every frame of a fight, to draw the aura round whoever is
	// up, so the answer lives beside the drawing and this reads it -- and a line
	// with no creature behind it comes back empty from there rather than being
	// tested for twice. See actingPawnIds in render/scene.ts.
	const acting = actingPawnIds(state.initiative);
	if (acting.length === 0) {
		return null;
	}

	const cell = state.table.grid.cellSize;
	let box: Rect | null = null;

	for (const pawn of state.pawns) {
		if (pawn.layerId !== viewed || !acting.includes(pawn.id)) {
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
