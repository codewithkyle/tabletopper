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

import type { Event } from "./protocol.ts";
import { PAWN_WINDOW } from "./pawn-window.ts";

// panelEvents is which DOM event each family of protocol events raises. A
// family that no panel listens for is simply absent.
const panelEvents: Partial<Record<Event["type"], string>> = {
	"player.joined": "room:players",
	"player.updated": "room:players",
	"player.left": "room:players",
	"initiative.updated": "room:initiative",
	"room.updated": "room:info",
	"table.updated": "room:tabletop",
};

// A snapshot changes everything at once, so it raises everything at once. This
// is the first join and every resync, which is exactly when a panel drawn from
// a stale fetch would be wrong.
const everything = ["room:players", "room:initiative", "room:info", "room:tabletop"];

export function announce(event: Event): void {
	if (event.type === "snapshot") {
		for (const name of everything) {
			window.dispatchEvent(new CustomEvent(name));
		}

		return;
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
				new CustomEvent("window:retitle", {
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
				new CustomEvent("window:close", { detail: { id: PAWN_WINDOW + event.id } }),
			);

			return;
	}

	const name = panelEvents[event.type];
	if (name) {
		window.dispatchEvent(new CustomEvent(name));
	}
}

function pawnChanged(id: string): void {
	window.dispatchEvent(new CustomEvent("room:pawn", { detail: { id } }));
}
