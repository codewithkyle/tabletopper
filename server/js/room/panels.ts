// The bridge from the socket to the DOM panels, and it is a mapping table
// because that is the whole of it.
//
// LIVE PANELS UPDATE BY REFETCH. A socket event fires a DOM event on window, an
// element's hx-trigger hears it, and htmx fetches a room fragment that reads
// the in-memory room. Six clients doing one GET per hit-point change is
// nothing, and it keeps rendered markup on HTTP where every other page in this
// app keeps it -- the socket carries JSON and nothing else.
//
// A PANEL THEREFORE NEEDS NO JAVASCRIPT OF ITS OWN. It declares what it listens
// for in its own markup:
//
//     hx-get="/fragment/room/members?room=..." hx-trigger="room:players from:window"
//
// THIS IS ALSO WHY EVERY TABLE MUTATION ANSWERS 204. The layer manager, the
// grid form and the layer name in the bar all listen for room:tabletop, and every
// one of the GM's controls ends in table.updated -- so the window that sent the
// command and the one open in another tab are corrected by the same event, from
// the same source, at the same time. A reply carrying the new markup would
// correct one of them.
//
// ONLY EVENTS THAT COUNT, AND NEVER A HOT PATH. Hit points, armour class,
// conditions, names and layers change at human pace -- a few per round -- and a
// GET each is nothing. pawn.moved, pawn.dragging and stroke.extended fire up to
// twenty times a second, and binding a refetch to one of them is precisely what
// "the socket carries JSON and the DOM refetches" was designed to avoid. They
// are absent from the table below and panels.test.ts asserts that they raise
// nothing at all, so the rule fails where it would be broken.
//
// A window that wants a per-frame value does not fetch it: the client already
// holds the whole projected room in store.ts, which is what the debug panel
// reads. That escape hatch exists and needs no change to the protocol.

import {
	ROOM_INFO,
	ROOM_INITIATIVE,
	ROOM_PAWN,
	ROOM_PLAYERS,
	ROOM_TABLETOP,
	WINDOW_CLOSE,
	WINDOW_RETITLE,
} from "../../public/js/events.js";
import type { Event, InitiativeEntry, Pawn } from "./protocol.ts";
import { PAWN_WINDOW } from "./pawn-window.ts";
import { openWindows } from "./window.ts";

// tracked is the set of pawn ids the turn order names, and it is the one piece
// of state this file holds.
//
// THE STRIP LISTENS FOR room:initiative AND DAMAGE ARRIVES AS pawn.updated. A
// line of the turn order wears its creature's wounds -- blood, pallor, a pulse,
// a skull -- and every one of those is read off hit points that change through
// the pawn family, which raises room:pawn with an id and nothing else. Without
// this, the blood on the strip would be stale until the turn advanced, which is
// the one thing that treatment cannot afford.
//
// IT IS THE PANEL-SIDE MIRROR OF THE FILTER EACH PAWN WINDOW CARRIES. A window
// cares about one id and says so in its own markup; a strip cares about a SET,
// and a set is not something an hx-trigger filter can hold -- so it is asked for
// here instead.
//
// IT STAYS INSIDE THE HOT-PATH RULE. Hit points, conditions and names change a
// few times a round. pawn.moved and pawn.dragging are not in the switch below
// and panels.test.ts asserts they raise nothing at all.
let tracked = new Set<string>();

// panelEvents is which DOM event each family of protocol events raises. A
// family that no panel listens for is simply absent.
const panelEvents: Partial<Record<Event["type"], string>> = {
	"player.joined": ROOM_PLAYERS,
	"player.updated": ROOM_PLAYERS,
	"player.left": ROOM_PLAYERS,
	"initiative.updated": ROOM_INITIATIVE,
	"room.updated": ROOM_INFO,
	"table.updated": ROOM_TABLETOP,
};

// A snapshot changes everything at once, so it raises everything at once. This
// is the first join and every resync, which is exactly when a panel drawn from
// a stale fetch would be wrong.
const everything = [ROOM_PLAYERS, ROOM_INITIATIVE, ROOM_INFO, ROOM_TABLETOP];

export function announce(event: Event): void {
	if (event.type === "snapshot") {
		track(event.state.initiative.entries);

		for (const name of everything) {
			window.dispatchEvent(new CustomEvent(name));
		}

		reconcilePawnWindows(event.state.pawns);

		return;
	}

	if (event.type === "initiative.updated") {
		track(event.initiative.entries);
	}

	// THE PAWN EVENTS CARRY AN ID AND THE REST DO NOT, and the id is what makes
	// a table full of open pawn windows affordable. Every one of them hears
	// room:pawn; the filter in each panel's own hx-trigger compares the id and
	// all but one decline. Without it, one goblin taking damage is a GET per
	// open window, every round, for the whole fight.
	switch (event.type) {
		case "pawn.spawned":
		case "pawn.updated":
			pawnChanged(event.pawn.id);

			// A window's heading is set from the trigger that opened it, so a
			// rename reaches the body and not the bar. This is the other half.
			window.dispatchEvent(
				new CustomEvent(WINDOW_RETITLE, {
					detail: { id: PAWN_WINDOW + event.pawn.id, title: event.pawn.name },
				}),
			);

			return;

		case "pawn.removed":
			pawnChanged(event.id);

			// The panel's fragment 404s from here on and the page's noSwap
			// config covers 4xx, so nothing would swap and the window would sit
			// there showing a dead goblin's hit points. The close has to come
			// from outside the fragment, because the fragment is what stopped
			// existing.
			window.dispatchEvent(
				new CustomEvent(WINDOW_CLOSE, { detail: { id: PAWN_WINDOW + event.id } }),
			);

			return;
	}

	const name = panelEvents[event.type];
	if (name) {
		window.dispatchEvent(new CustomEvent(name));
	}
}

// reconcilePawnWindows is the snapshot reaching the pawn windows, which listen
// for one id each and would otherwise hear nothing from it.
//
// AFTER A RECONNECT EVERY OPEN PAWN WINDOW WAS STALE. A laptop lid shut for
// three rounds comes back to a snapshot that raises the four panel events and
// no room:pawn -- so each window went on showing pre-disconnect hit points and
// conditions until that one pawn next changed. And a pawn removed while the
// tab was away left its window open for good: the fragment 404s, noSwap keeps
// the stale markup, and the window is remembered on every save. So each open
// pawn window is either told to refetch or told to close, by whether its pawn
// is in the state that just arrived.
function reconcilePawnWindows(pawns: readonly Pawn[]): void {
	let present: Set<string> | null = null;

	for (const id of openWindows()) {
		if (!id.startsWith(PAWN_WINDOW)) {
			continue;
		}

		present ??= new Set(pawns.map((p) => p.id));

		const pawn = id.slice(PAWN_WINDOW.length);
		if (present.has(pawn)) {
			window.dispatchEvent(new CustomEvent(ROOM_PAWN, { detail: { id: pawn } }));
		} else {
			window.dispatchEvent(new CustomEvent(WINDOW_CLOSE, { detail: { id } }));
		}
	}
}

function pawnChanged(id: string): void {
	window.dispatchEvent(new CustomEvent(ROOM_PAWN, { detail: { id } }));

	if (tracked.has(id)) {
		window.dispatchEvent(new CustomEvent(ROOM_INITIATIVE));
	}
}

// track refreshes the set from a tracker that has just arrived. It is rebuilt
// rather than added to, because a line leaving the order is exactly as
// interesting as one joining it: a goblin taken out of the tracker should stop
// costing a refetch every time it is hit.
function track(entries: readonly InitiativeEntry[]): void {
	const ids = new Set<string>();
	for (const entry of entries) {
		for (const id of entry.pawnIds) {
			ids.add(id);
		}
	}

	tracked = ids;
}
